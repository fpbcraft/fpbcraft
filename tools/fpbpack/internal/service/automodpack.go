package service

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/automodpack"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

const autoModpackConfigPath = "automodpack/server.conf"

type AutoModpackPublishedFile struct {
	Path     string `json:"path"`
	Size     string `json:"size,omitempty"`
	Type     string `json:"type,omitempty"`
	Editable bool   `json:"editable,omitempty"`
	SHA1     string `json:"sha1,omitempty"`
}

type AutoModpackGroupStatus struct {
	ID                 string                     `json:"id"`
	Category           string                     `json:"category"`
	Path               string                     `json:"path"`
	Exists             bool                       `json:"exists"`
	Files              int                        `json:"files"`
	Mods               int                        `json:"mods"`
	Bytes              int64                      `json:"bytes"`
	PublishedFiles     []AutoModpackPublishedFile `json:"published_files,omitempty"`
	PublishedTruncated bool                       `json:"published_truncated,omitempty"`
}

type AutoModpackGenerationSummary struct {
	Added   int `json:"added"`
	Changed int `json:"changed"`
	Removed int `json:"removed"`
}

type AutoModpackGeneration struct {
	Sequence     int64                        `json:"sequence"`
	ContentToken string                       `json:"content_token"`
	CreatedAt    time.Time                    `json:"created_at"`
	Notes        string                       `json:"notes,omitempty"`
	RestoreOf    int64                        `json:"restore_of,omitempty"`
	Summary      AutoModpackGenerationSummary `json:"summary"`
}

type AutoModpackStatus struct {
	Installed              bool                       `json:"installed"`
	Version                string                     `json:"version,omitempty"`
	JAR                    string                     `json:"jar,omitempty"`
	ConfigPresent          bool                       `json:"config_present"`
	ConfigPath             string                     `json:"config_path"`
	ConfigSHA256           string                     `json:"config_sha256,omitempty"`
	Config                 automodpack.Config         `json:"config"`
	Findings               []automodpack.Finding      `json:"findings"`
	Groups                 []AutoModpackGroupStatus   `json:"groups"`
	OrphanGroupDirectories []string                   `json:"orphan_group_directories,omitempty"`
	PendingPublish          bool                       `json:"pending_publish"`
	LastChangedAt          *time.Time                 `json:"last_changed_at,omitempty"`
	LastPublishRequestedAt *time.Time                 `json:"last_publish_requested_at,omitempty"`
	Generations            []AutoModpackGeneration    `json:"generations"`
	PublishedContentToken  string                     `json:"published_content_token,omitempty"`
	PublishedJournalHead   int64                      `json:"published_journal_head,omitempty"`
}

type AutoModpackConfigRequest struct {
	ExpectedSHA256        string             `json:"expected_sha256"`
	Config                automodpack.Config `json:"config"`
	ConfirmIdentityChanges bool              `json:"confirm_identity_changes,omitempty"`
}

type AutoModpackGroupMigrationRequest struct {
	ExpectedSHA256 string `json:"expected_sha256"`
	OldID          string `json:"old_id"`
	NewID          string `json:"new_id"`
}

func (s *Service) AutoModpackStatus() (AutoModpackStatus, error) {
	s.mu.RLock()
	managedState := s.state.AutoModpack
	snapshot := s.snapshot
	catalogEntries := append([]catalogEntryForAutoModpack(nil), catalogEntriesForAutoModpack(s.state.Catalog.Managed)...)
	s.mu.RUnlock()

	status := AutoModpackStatus{
		ConfigPath:             autoModpackConfigPath,
		Config:                 automodpack.DefaultConfig(),
		Findings:               []automodpack.Finding{},
		Groups:                 []AutoModpackGroupStatus{},
		PendingPublish:         managedState.PendingPublish,
		LastChangedAt:          managedState.LastChangedAt,
		LastPublishRequestedAt: managedState.LastPublishRequestedAt,
		Generations:            []AutoModpackGeneration{},
	}
	for _, mod := range snapshot.Inventory.Mods {
		detected := false
		for _, metadata := range mod.Metadata {
			if strings.EqualFold(strings.TrimSpace(metadata.ModID), "automodpack") {
				status.Installed = true
				status.JAR = mod.Path
				status.Version = metadata.Version
				detected = true
				break
			}
		}
		if !detected && strings.Contains(strings.ToLower(mod.Filename), "automodpack") {
			status.Installed = true
			status.JAR = mod.Path
		}
	}

	doc, content, err := s.readAutoModpackDocument()
	if err != nil {
		if os.IsNotExist(err) {
			status.Findings = append(status.Findings, automodpack.Finding{
				Level: "warning", Code: "config_missing",
				Message: "automodpack/server.conf does not exist yet. Start AutoModpack once to create its server configuration.",
			})
			return status, nil
		}
		return AutoModpackStatus{}, err
	}
	status.ConfigPresent = true
	sum := sha256.Sum256(content)
	status.ConfigSHA256 = hex.EncodeToString(sum[:])
	status.Config = doc.Config()
	status.Findings = append(status.Findings, automodpack.Validate(status.Config)...)
	if status.Config.Settings.SelfUpdater {
		status.Findings = append(status.Findings, automodpack.Finding{
			Level: "warning", Code: "self_updater_enabled",
			Message: "AutoModpack self-updater is enabled. FPBPack may report drift if AutoModpack replaces its own managed JAR.",
		})
	}

	configuredGroups := map[string]automodpack.Group{}
	for _, category := range status.Config.Categories {
		for _, group := range category.Groups {
			configuredGroups[group.ID] = group
			groupStatus, statErr := s.autoModpackGroupStatus(category.Name, group.ID)
			if statErr != nil {
				status.Findings = append(status.Findings, automodpack.Finding{
					Level: "warning", Code: "group_scan_failed", Group: group.ID,
					Message: fmt.Sprintf("Could not inspect AutoModpack group %q: %v", group.ID, statErr),
				})
			} else {
				status.Groups = append(status.Groups, groupStatus)
			}
		}
	}
	for _, entry := range catalogEntries {
		if entry.Group == "" {
			continue
		}
		if _, exists := configuredGroups[entry.Group]; !exists {
			status.Findings = append(status.Findings, automodpack.Finding{
				Level: "error", Code: "managed_group_missing", Group: entry.Group,
				Message: fmt.Sprintf("Managed client artifact %s targets AutoModpack group %q, but that group is not declared in server.conf.", entry.Name, entry.Group),
			})
		}
	}

	orphanDirectories, scanErr := s.autoModpackOrphanDirectories(configuredGroups)
	if scanErr != nil {
		status.Findings = append(status.Findings, automodpack.Finding{
			Level: "warning", Code: "group_directory_scan_failed", Message: scanErr.Error(),
		})
	} else {
		status.OrphanGroupDirectories = orphanDirectories
		for _, orphan := range orphanDirectories {
			status.Findings = append(status.Findings, automodpack.Finding{
				Level: "warning", Code: "orphan_group_directory", Group: orphan,
				Message: fmt.Sprintf("host-modpack/%s exists but server.conf does not declare group %q.", orphan, orphan),
			})
		}
	}
	status.Findings = append(status.Findings, s.autoModpackDirectContentCollisions(configuredGroups)...)
	publishedGroups, contentToken, journalHead, projectionErr := s.autoModpackPublishedContent()
	if projectionErr != nil {
		status.Findings = append(status.Findings, automodpack.Finding{
			Level: "warning", Code: "published_projection_unreadable",
			Message: fmt.Sprintf("Could not read AutoModpack current published projection: %v", projectionErr),
		})
	} else {
		status.PublishedContentToken = contentToken
		status.PublishedJournalHead = journalHead
		for index := range status.Groups {
			files := publishedGroups[status.Groups[index].ID]
			if len(files) > 500 {
				status.Groups[index].PublishedFiles = append([]AutoModpackPublishedFile(nil), files[:500]...)
				status.Groups[index].PublishedTruncated = true
			} else {
				status.Groups[index].PublishedFiles = append([]AutoModpackPublishedFile(nil), files...)
			}
		}
	}
	generations, historyErr := s.autoModpackGenerationHistory()
	if historyErr != nil {
		status.Findings = append(status.Findings, automodpack.Finding{
			Level: "warning", Code: "generation_history_unreadable",
			Message: fmt.Sprintf("Could not read AutoModpack generation history: %v", historyErr),
		})
	} else {
		status.Generations = generations
	}
	return status, nil
}

type catalogEntryForAutoModpack struct {
	Name  string
	Group string
}

func catalogEntriesForAutoModpack(entries []catalog.Entry) []catalogEntryForAutoModpack {
	result := make([]catalogEntryForAutoModpack, 0)
	for _, entry := range entries {
		if entry.Deployment != inventory.LocationClient {
			continue
		}
		result = append(result, catalogEntryForAutoModpack{
			Name: entry.Name,
			Group: normalizeAutoModpackGroup(entry.Deployment, entry.AutoModpackGroup),
		})
	}
	return result
}

func (s *Service) readAutoModpackDocument() (*automodpack.Document, []byte, error) {
	path := filepath.Join(s.options.ServerRoot, filepath.FromSlash(autoModpackConfigPath))
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	doc, err := automodpack.Parse(content)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", autoModpackConfigPath, err)
	}
	return doc, content, nil
}

func (s *Service) UpdateAutoModpackConfig(_ context.Context, request AutoModpackConfigRequest) (AutoModpackStatus, error) {
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	doc, content, err := s.readAutoModpackDocument()
	if err != nil {
		return AutoModpackStatus{}, err
	}
	currentSum := sha256.Sum256(content)
	currentHash := hex.EncodeToString(currentSum[:])
	if strings.TrimSpace(request.ExpectedSHA256) == "" || !strings.EqualFold(strings.TrimSpace(request.ExpectedSHA256), currentHash) {
		return AutoModpackStatus{}, fmt.Errorf("AutoModpack configuration changed since it was loaded; reload the page before saving")
	}
	current := doc.Config()
	if err := s.validateAutoModpackIdentityChanges(current, request.Config, request.ConfirmIdentityChanges); err != nil {
		return AutoModpackStatus{}, err
	}
	if err := doc.Apply(request.Config); err != nil {
		return AutoModpackStatus{}, fmt.Errorf("validate AutoModpack configuration: %w", err)
	}
	next := doc.Render()
	if err := s.backupAutoModpackConfig(content, currentHash); err != nil {
		return AutoModpackStatus{}, fmt.Errorf("backup AutoModpack configuration: %w", err)
	}
	path := filepath.Join(s.options.ServerRoot, filepath.FromSlash(autoModpackConfigPath))
	if err := writeBytesAtomic(path, next, 0o644); err != nil {
		return AutoModpackStatus{}, fmt.Errorf("write AutoModpack configuration: %w", err)
	}
	if err := s.markAutoModpackChanged(); err != nil {
		return AutoModpackStatus{}, fmt.Errorf("persist AutoModpack publication state: %w", err)
	}
	s.logEvent("info", "automodpack", "Saved automodpack/server.conf; configuration reload is still required on a running server")
	return s.AutoModpackStatus()
}

func (s *Service) validateAutoModpackIdentityChanges(current, next automodpack.Config, confirmed bool) error {
	oldCategories := map[string]struct{}{}
	newCategories := map[string]struct{}{}
	oldGroups := map[string]struct{}{}
	newGroups := map[string]struct{}{}
	for _, category := range current.Categories {
		oldCategories[category.Name] = struct{}{}
		for _, group := range category.Groups {
			oldGroups[group.ID] = struct{}{}
		}
	}
	for _, category := range next.Categories {
		newCategories[category.Name] = struct{}{}
		for _, group := range category.Groups {
			newGroups[group.ID] = struct{}{}
		}
	}
	for category := range oldCategories {
		if _, exists := newCategories[category]; !exists && !confirmed {
			return fmt.Errorf("removing or renaming category %q changes players' saved group-selection identity; retry with identity-change confirmation", category)
		}
	}
	s.mu.RLock()
	managedEntries := append([]catalog.Entry(nil), s.state.Catalog.Managed...)
	s.mu.RUnlock()
	for group := range oldGroups {
		if _, exists := newGroups[group]; exists {
			continue
		}
		for _, entry := range managedEntries {
			if entry.Deployment == inventory.LocationClient &&
				normalizeAutoModpackGroup(entry.Deployment, entry.AutoModpackGroup) == group {
				return fmt.Errorf("group %q is still the preferred destination for managed artifact %q; move or reassign it before deleting the group", group, entry.Name)
			}
		}
		groupPath := filepath.Join(s.options.ServerRoot, filepath.FromSlash(inventory.DefaultAutoModpackHostPath), group)
		if nonEmpty, err := directoryHasContent(groupPath); err != nil {
			return err
		} else if nonEmpty {
			return fmt.Errorf("group %q still has content under host-modpack/%s; move/delete the content or use the identity migration action instead", group, group)
		}
		if !confirmed {
			return fmt.Errorf("removing group %q changes players' saved group-selection identity; retry with identity-change confirmation", group)
		}
	}
	return nil
}

func (s *Service) MigrateAutoModpackGroup(ctx context.Context, request AutoModpackGroupMigrationRequest) (AutoModpackStatus, error) {
	if err := s.requireServerStopped(ctx); err != nil {
		return AutoModpackStatus{}, err
	}
	oldID := strings.TrimSpace(request.OldID)
	newID := strings.TrimSpace(request.NewID)
	if err := validateAutoModpackGroupID(newID); err != nil {
		return AutoModpackStatus{}, err
	}
	if oldID == "" || oldID == newID {
		return AutoModpackStatus{}, fmt.Errorf("old and new AutoModpack group ids must be different")
	}

	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()
	doc, content, err := s.readAutoModpackDocument()
	if err != nil {
		return AutoModpackStatus{}, err
	}
	sum := sha256.Sum256(content)
	currentHash := hex.EncodeToString(sum[:])
	if strings.TrimSpace(request.ExpectedSHA256) == "" || !strings.EqualFold(request.ExpectedSHA256, currentHash) {
		return AutoModpackStatus{}, fmt.Errorf("AutoModpack configuration changed since it was loaded; reload before migrating the group")
	}
	cfg := doc.Config()
	found := false
	for categoryIndex := range cfg.Categories {
		for groupIndex := range cfg.Categories[categoryIndex].Groups {
			group := &cfg.Categories[categoryIndex].Groups[groupIndex]
			if group.ID == newID {
				return AutoModpackStatus{}, fmt.Errorf("AutoModpack group %q already exists", newID)
			}
			if group.ID == oldID {
				group.ID = newID
				found = true
			}
		}
	}
	if !found {
		return AutoModpackStatus{}, fmt.Errorf("AutoModpack group %q does not exist", oldID)
	}
	for categoryIndex := range cfg.Categories {
		for groupIndex := range cfg.Categories[categoryIndex].Groups {
			group := &cfg.Categories[categoryIndex].Groups[groupIndex]
			group.Requires = replaceString(group.Requires, oldID, newID)
			group.BreaksWith = replaceString(group.BreaksWith, oldID, newID)
		}
	}
	if err := doc.Apply(cfg); err != nil {
		return AutoModpackStatus{}, err
	}

	oldDir := filepath.Join(s.options.ServerRoot, filepath.FromSlash(inventory.DefaultAutoModpackHostPath), oldID)
	newDir := filepath.Join(s.options.ServerRoot, filepath.FromSlash(inventory.DefaultAutoModpackHostPath), newID)
	renamed := false
	if _, statErr := os.Stat(oldDir); statErr == nil {
		if _, targetErr := os.Stat(newDir); targetErr == nil {
			return AutoModpackStatus{}, fmt.Errorf("target group directory already exists: host-modpack/%s", newID)
		} else if !os.IsNotExist(targetErr) {
			return AutoModpackStatus{}, targetErr
		}
		if err := os.Rename(oldDir, newDir); err != nil {
			return AutoModpackStatus{}, fmt.Errorf("rename AutoModpack group directory: %w", err)
		}
		renamed = true
	} else if !os.IsNotExist(statErr) {
		return AutoModpackStatus{}, statErr
	}
	if renamed {
		defer func() {
			if _, err := os.Stat(oldDir); os.IsNotExist(err) {
				// The config write succeeded when the new directory remains in place.
			}
		}()
	}

	if err := s.backupAutoModpackConfig(content, currentHash); err != nil {
		if renamed {
			_ = os.Rename(newDir, oldDir)
		}
		return AutoModpackStatus{}, err
	}
	configPath := filepath.Join(s.options.ServerRoot, filepath.FromSlash(autoModpackConfigPath))
	if err := writeBytesAtomic(configPath, doc.Render(), 0o644); err != nil {
		if renamed {
			_ = os.Rename(newDir, oldDir)
		}
		return AutoModpackStatus{}, err
	}

	s.mu.Lock()
	for index := range s.state.Catalog.Managed {
		entry := &s.state.Catalog.Managed[index]
		if entry.Deployment == inventory.LocationClient &&
			normalizeAutoModpackGroup(entry.Deployment, entry.AutoModpackGroup) == oldID {
			entry.AutoModpackGroup = newID
			for sourceIndex := range entry.SourcePaths {
				source := &entry.SourcePaths[sourceIndex]
				if source.Location != inventory.LocationClient ||
					normalizeAutoModpackGroup(source.Location, source.Group) != oldID {
					continue
				}
				source.Group = newID
				oldPrefix := filepath.ToSlash(filepath.Join(inventory.DefaultAutoModpackHostPath, oldID)) + "/"
				newPrefix := filepath.ToSlash(filepath.Join(inventory.DefaultAutoModpackHostPath, newID)) + "/"
				if strings.HasPrefix(filepath.ToSlash(source.Path), oldPrefix) {
					source.Path = newPrefix + strings.TrimPrefix(filepath.ToSlash(source.Path), oldPrefix)
				}
			}
		}
	}
	s.state.Catalog.RecalculateSummary()
	now := time.Now().UTC()
	s.state.AutoModpack.PendingPublish = true
	s.state.AutoModpack.LastChangedAt = &now
	s.state.UpdatedAt = now
	persistErr := s.persistState()
	s.mu.Unlock()
	if persistErr != nil {
		if renamed {
			_ = os.Rename(newDir, oldDir)
		}
		_ = writeBytesAtomic(configPath, content, 0o644)
		return AutoModpackStatus{}, fmt.Errorf("persist migrated FPBPack group identity: %w", persistErr)
	}

	s.logEvent("info", "automodpack", fmt.Sprintf("Migrated AutoModpack group identity %s → %s", oldID, newID))
	return s.AutoModpackStatus()
}

func (s *Service) autoModpackGroupStatus(category, group string) (AutoModpackGroupStatus, error) {
	relative := filepath.ToSlash(filepath.Join(inventory.DefaultAutoModpackHostPath, group))
	path := filepath.Join(s.options.ServerRoot, filepath.FromSlash(relative))
	result := AutoModpackGroupStatus{ID: group, Category: category, Path: relative}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if !info.IsDir() {
		return result, fmt.Errorf("%s is not a directory", relative)
	}
	result.Exists = true
	err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		result.Files++
		result.Bytes += info.Size()
		if strings.EqualFold(filepath.Ext(entry.Name()), ".jar") {
			result.Mods++
		}
		return nil
	})
	return result, err
}

func (s *Service) autoModpackOrphanDirectories(configured map[string]automodpack.Group) ([]string, error) {
	root := filepath.Join(s.options.ServerRoot, filepath.FromSlash(inventory.DefaultAutoModpackHostPath))
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read AutoModpack host-modpack directory: %w", err)
	}
	result := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, exists := configured[entry.Name()]; !exists {
			result = append(result, entry.Name())
		}
	}
	sort.Strings(result)
	return result, nil
}

func (s *Service) autoModpackDirectContentCollisions(groups map[string]automodpack.Group) []automodpack.Finding {
	type owner struct {
		group string
		path  string
	}
	owners := map[string]owner{}
	findings := []automodpack.Finding{}
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		root := filepath.Join(s.options.ServerRoot, filepath.FromSlash(inventory.DefaultAutoModpackHostPath), id)
		_ = filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			relative, relErr := filepath.Rel(root, current)
			if relErr != nil {
				return nil
			}
			key := strings.ToLower(filepath.ToSlash(relative))
			if previous, exists := owners[key]; exists && !groupsConflict(groups[previous.group], id) && !groupsConflict(groups[id], previous.group) {
				findings = append(findings, automodpack.Finding{
					Level: "warning", Code: "direct_content_collision", Group: id,
					Message: fmt.Sprintf("Groups %q and %q both directly contain %s; verify they cannot be selected together or exclude one copy.", previous.group, id, filepath.ToSlash(relative)),
				})
			} else if !exists {
				owners[key] = owner{group: id, path: filepath.ToSlash(relative)}
			}
			return nil
		})
	}
	return findings
}

func groupsConflict(group automodpack.Group, other string) bool {
	for _, candidate := range group.BreaksWith {
		if candidate == other {
			return true
		}
	}
	return false
}

func (s *Service) backupAutoModpackConfig(content []byte, hash string) error {
	dir := filepath.Join(s.options.StateDir, "automodpack", "config-backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := time.Now().UTC().Format("20060102T150405.000000000Z")
	if len(hash) >= 12 {
		name += "-" + hash[:12]
	}
	if err := os.WriteFile(filepath.Join(dir, name+"-server.conf"), content, 0o600); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	retention := s.Settings().RetentionCount
	if retention < 1 {
		retention = DefaultRetentionCount
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	if len(entries) > retention {
		for _, entry := range entries[retention:] {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
	return nil
}

func (s *Service) markAutoModpackChanged() error {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.AutoModpack.PendingPublish = true
	s.state.AutoModpack.LastChangedAt = &now
	s.state.UpdatedAt = now
	return s.persistState()
}

func directoryHasContent(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

func replaceString(items []string, oldValue, newValue string) []string {
	result := append([]string(nil), items...)
	for index := range result {
		if result[index] == oldValue {
			result[index] = newValue
		}
	}
	return result
}

func writeBytesAtomic(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".fpbpack-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}


func (s *Service) autoModpackGenerationHistory() ([]AutoModpackGeneration, error) {
	path := filepath.Join(s.options.ServerRoot, "automodpack", "server", "journal.jsonl")
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return []AutoModpackGeneration{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	type journalChange struct {
		Path     string `json:"path"`
		FromSHA1 string `json:"fromSha1"`
		ToSHA1   string `json:"toSha1"`
	}
	type journalEntry struct {
		Seq          int64           `json:"seq"`
		ContentToken string          `json:"contentToken"`
		CreatedAt    string          `json:"createdAt"`
		Notes        string          `json:"notes"`
		RestoreOf    int64           `json:"restoreOf"`
		Changes      []journalChange `json:"changes"`
	}

	history := []AutoModpackGeneration{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var entry journalEntry
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			// AutoModpack tolerates a torn final journal line after a crash. Keep
			// earlier durable entries usable and ignore only that final fragment.
			if !scanner.Scan() {
				break
			}
			return nil, fmt.Errorf("invalid journal entry at line %d: %w", line, err)
		}
		createdAt, err := time.Parse(time.RFC3339Nano, entry.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("invalid journal timestamp at line %d: %w", line, err)
		}
		summary := AutoModpackGenerationSummary{}
		for _, change := range entry.Changes {
			from := strings.TrimSpace(change.FromSHA1)
			to := strings.TrimSpace(change.ToSHA1)
			switch {
			case from == "" && to != "":
				summary.Added++
			case from != "" && to == "":
				summary.Removed++
			default:
				summary.Changed++
			}
		}
		history = append(history, AutoModpackGeneration{
			Sequence: entry.Seq,
			ContentToken: entry.ContentToken,
			CreatedAt: createdAt.UTC(),
			Notes: entry.Notes,
			RestoreOf: entry.RestoreOf,
			Summary: summary,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(history, func(i, j int) bool { return history[i].Sequence > history[j].Sequence })
	if len(history) > 100 {
		history = history[:100]
	}
	return history, nil
}


func (s *Service) autoModpackPublishedContent() (map[string][]AutoModpackPublishedFile, string, int64, error) {
	path := filepath.Join(s.options.ServerRoot, "automodpack", "server", "current-projection.json")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string][]AutoModpackPublishedFile{}, "", 0, nil
	}
	if err != nil {
		return nil, "", 0, err
	}
	type groupFile struct {
		Size     string `json:"size"`
		Type     string `json:"type"`
		Editable bool   `json:"editable"`
		SHA1     string `json:"sha1"`
	}
	type group struct {
		Files map[string]groupFile `json:"files"`
	}
	var head struct {
		ContentToken string `json:"contentToken"`
		JournalHead  int64  `json:"journalHead"`
		Policy struct {
			Categories map[string]map[string]group `json:"categories"`
		} `json:"policy"`
	}
	if err := json.Unmarshal(content, &head); err != nil {
		return nil, "", 0, err
	}
	result := map[string][]AutoModpackPublishedFile{}
	for _, groups := range head.Policy.Categories {
		for groupID, groupData := range groups {
			files := make([]AutoModpackPublishedFile, 0, len(groupData.Files))
			for logicalPath, file := range groupData.Files {
				files = append(files, AutoModpackPublishedFile{
					Path: logicalPath,
					Size: file.Size,
					Type: file.Type,
					Editable: file.Editable,
					SHA1: file.SHA1,
				})
			}
			sort.Slice(files, func(i, j int) bool {
				return strings.ToLower(files[i].Path) < strings.ToLower(files[j].Path)
			})
			result[groupID] = files
		}
	}
	return result, head.ContentToken, head.JournalHead, nil
}
