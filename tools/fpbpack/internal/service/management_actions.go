package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

type ModManagementRequest struct {
	Action     string `json:"action"`
	Path       string `json:"path"`
	ProjectID  string `json:"project_id,omitempty"`
	VersionID  string `json:"version_id,omitempty"`
	FileID     uint32 `json:"file_id,omitempty"`
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Asset      string `json:"asset,omitempty"`
}

type ModManagementResult struct {
	Action     string `json:"action"`
	Path       string `json:"path"`
	Management string `json:"management,omitempty"`
	Provider   string `json:"provider,omitempty"`
	ProjectID  string `json:"project_id,omitempty"`
	Message    string `json:"message"`
}

func (s *Service) ManageMod(ctx context.Context, request ModManagementRequest) (ModManagementResult, error) {
	request.Action = strings.TrimSpace(request.Action)
	request.Path = normalizeCatalogPath(request.Path)
	if request.Path == "" {
		return ModManagementResult{}, fmt.Errorf("mod path is required")
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	switch request.Action {
	case "mark_unmanaged":
		return s.markModUnmanaged(request.Path)
	case "assign_modrinth":
		return s.assignModrinthSource(ctx, request)
	case "assign_curseforge":
		return s.assignCurseForgeSource(ctx, request)
	case "assign_github":
		return s.assignGitHubSource(ctx, request)
	case "forget_missing":
		return s.forgetMissingAcceptedEntry(request.Path)
	default:
		return ModManagementResult{}, fmt.Errorf("unsupported mod management action %q", request.Action)
	}
}

func (s *Service) RefreshModMetadata(ctx context.Context, path string) (err error) {
	path = normalizeCatalogPath(path)
	if path == "" {
		return fmt.Errorf("mod path is required")
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.beginRefresh()
	defer func() { s.finishRefresh(err) }()

	mod, ok := s.liveModByPath(path)
	if !ok {
		return fmt.Errorf("live mod %q was not found", path)
	}
	if s.isPinnedArtifact(mod) {
		return fmt.Errorf("this artifact is intentionally unmanaged; clear that decision or assign a source first")
	}

	if entry, ok := s.managedEntryByPath(path); ok {
		return s.refreshSingleManagedEntry(ctx, entry)
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	matches, lookupErr := (inventory.ModrinthClient{BaseURL: s.options.ModrinthBaseURL}).Match(
		lookupCtx,
		[]inventory.ModFile{mod},
	)
	if lookupErr != nil {
		return fmt.Errorf("refresh Modrinth identity: %w", lookupErr)
	}
	match, ok := matches[mod.SHA512]
	if !ok {
		return fmt.Errorf("no exact Modrinth source was found; assign a Modrinth, CurseForge, or GitHub source manually, or keep this artifact unmanaged")
	}

	copyMod := mod
	copyMod.Modrinth = &match
	synthetic := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		GeneratedAt: time.Now().UTC(),
		ModrinthChecked: true,
		Mods: []inventory.ModFile{copyMod},
	}
	result, buildErr := catalog.Build(synthetic)
	if buildErr != nil {
		return fmt.Errorf("build detected source: %w", buildErr)
	}
	if len(result.Report.Managed) != 1 {
		return fmt.Errorf("exact Modrinth match did not produce one managed source")
	}
	entry := result.Report.Managed[0]
	entry.SourcePaths = s.sourcesForSHA(mod.SHA512)
	s.replaceCatalogArtifact(mod, entry, "")
	if err := s.persistCatalogMutation(); err != nil {
		return err
	}
	return s.refreshSingleManagedEntry(ctx, entry)
}

func (s *Service) markModUnmanaged(path string) (ModManagementResult, error) {
	mod, ok := s.liveModByPath(path)
	if !ok {
		return ModManagementResult{}, fmt.Errorf("live mod %q was not found", path)
	}

	s.removeCatalogArtifact(mod, "")
	s.state.Catalog.Pinned = append(s.state.Catalog.Pinned, catalog.PinnedArtifact{
		SHA512: mod.SHA512,
		Filename: mod.Filename,
		Reason: "user_marked_unmanaged",
		Sources: s.sourcesForSHA(mod.SHA512),
	})
	sort.Slice(s.state.Catalog.Pinned, func(i, j int) bool {
		return s.state.Catalog.Pinned[i].Filename < s.state.Catalog.Pinned[j].Filename
	})
	if err := s.persistCatalogMutation(); err != nil {
		return ModManagementResult{}, err
	}
	return ModManagementResult{
		Action: "mark_unmanaged",
		Path: path,
		Management: "unmanaged",
		Message: "Artifact is now explicitly unmanaged and excluded from update planning.",
	}, nil
}

func (s *Service) assignModrinthSource(
	ctx context.Context,
	request ModManagementRequest,
) (ModManagementResult, error) {
	mod, ok := s.liveModByPath(request.Path)
	if !ok {
		return ModManagementResult{}, fmt.Errorf("live mod %q was not found", request.Path)
	}
	projectID := strings.TrimSpace(request.ProjectID)
	versionID := strings.TrimSpace(request.VersionID)
	if projectID == "" || versionID == "" {
		return ModManagementResult{}, fmt.Errorf("Modrinth project ID and installed version ID are required")
	}

	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	client := &updatecheck.ModrinthClient{
		BaseURL: s.options.ModrinthBaseURL,
		Mode: updatecheck.RefreshModeInteractive,
	}
	verified, err := client.VerifyInstalledVersion(
		verifyCtx,
		projectID,
		versionID,
		mod.Filename,
		mod.SHA512,
	)
	if err != nil {
		return ModManagementResult{}, fmt.Errorf("verify Modrinth source: %w", err)
	}

	name := managementDisplayName(mod)
	if strings.TrimSpace(verified.VersionName) != "" {
		name = verified.VersionName
	}
	entry := catalog.Entry{
		Provider: "modrinth",
		ProjectID: projectID,
		VersionID: verified.VersionID,
		Name: name,
		Filename: mod.Filename,
		SHA1: mod.SHA1,
		SHA512: mod.SHA512,
		URL: verified.DownloadURL,
		Side: s.sideForPath(mod.Path),
		Deployment: mod.Location,
		Environment: verified.Environment,
		SourcePaths: s.sourcesForSHA(mod.SHA512),
	}
	s.replaceCatalogArtifact(mod, entry, "modrinth:"+projectID)
	if err := s.persistCatalogMutation(); err != nil {
		return ModManagementResult{}, err
	}
	message := "Modrinth source verified and accepted."
	if err := s.refreshSingleManagedEntry(ctx, entry); err != nil {
		message = "Modrinth source was verified and saved, but its update metadata refresh failed: " + err.Error()
	}
	return ModManagementResult{
		Action: "assign_modrinth",
		Path: request.Path,
		Management: "managed",
		Provider: "modrinth",
		ProjectID: projectID,
		Message: message,
	}, nil
}

func (s *Service) assignCurseForgeSource(
	ctx context.Context,
	request ModManagementRequest,
) (ModManagementResult, error) {
	mod, ok := s.liveModByPath(request.Path)
	if !ok {
		return ModManagementResult{}, fmt.Errorf("live mod %q was not found", request.Path)
	}
	projectID := strings.TrimSpace(request.ProjectID)
	if projectID == "" || request.FileID == 0 {
		return ModManagementResult{}, fmt.Errorf("CurseForge project ID and installed file ID are required")
	}
	apiKey, _ := s.effectiveCurseForgeAPIKey()
	if apiKey == "" {
		return ModManagementResult{}, fmt.Errorf("configure a CurseForge API key in Settings → Providers before assigning a CurseForge source")
	}

	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	client := &updatecheck.CurseForgeClient{
		BaseURL: s.options.CurseForgeBaseURL,
		APIKey: apiKey,
		Mode: updatecheck.RefreshModeInteractive,
	}
	verified, err := client.VerifyInstalledFile(
		verifyCtx,
		projectID,
		request.FileID,
		mod.SHA1,
	)
	if err != nil {
		return ModManagementResult{}, fmt.Errorf("verify CurseForge source: %w", err)
	}

	name := managementDisplayName(mod)
	if strings.TrimSpace(verified.DisplayName) != "" {
		name = verified.DisplayName
	}
	entry := catalog.Entry{
		Provider: "curseforge",
		ProjectID: projectID,
		FileID: request.FileID,
		Name: name,
		Filename: mod.Filename,
		SHA1: mod.SHA1,
		SHA512: mod.SHA512,
		Side: s.sideForPath(mod.Path),
		Deployment: mod.Location,
		SourcePaths: s.sourcesForSHA(mod.SHA512),
	}
	s.replaceCatalogArtifact(mod, entry, "curseforge:"+projectID)
	if err := s.persistCatalogMutation(); err != nil {
		return ModManagementResult{}, err
	}
	message := "CurseForge source verified and accepted."
	if err := s.refreshSingleManagedEntry(ctx, entry); err != nil {
		message = "CurseForge source was verified and saved, but its update metadata refresh failed: " + err.Error()
	}
	return ModManagementResult{
		Action: "assign_curseforge",
		Path: request.Path,
		Management: "managed",
		Provider: "curseforge",
		ProjectID: projectID,
		Message: message,
	}, nil
}

func (s *Service) assignGitHubSource(
	ctx context.Context,
	request ModManagementRequest,
) (ModManagementResult, error) {
	mod, ok := s.liveModByPath(request.Path)
	if !ok {
		return ModManagementResult{}, fmt.Errorf("live mod %q was not found", request.Path)
	}
	repository := strings.TrimSpace(request.Repository)
	tag := strings.TrimSpace(request.Tag)
	asset := strings.TrimSpace(request.Asset)
	if asset == "" {
		asset = mod.Filename
	}
	if repository == "" || tag == "" {
		return ModManagementResult{}, fmt.Errorf("GitHub repository and installed release tag are required")
	}

	livePath := filepath.Join(s.options.ServerRoot, filepath.FromSlash(mod.Path))
	sha256Value, err := sha256File(livePath)
	if err != nil {
		return ModManagementResult{}, fmt.Errorf("hash current JAR: %w", err)
	}

	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	client := &updatecheck.GitHubClient{
		BaseURL: s.options.GitHubBaseURL,
		Token: s.options.GitHubToken,
		Mode: updatecheck.RefreshModeInteractive,
	}
	verified, err := client.VerifyInstalledAsset(verifyCtx, repository, tag, asset, sha256Value)
	if err != nil {
		return ModManagementResult{}, fmt.Errorf("verify GitHub source: %w", err)
	}

	entry := catalog.Entry{
		Provider: "github",
		ProjectID: repository,
		VersionID: tag,
		Name: managementDisplayName(mod),
		Filename: mod.Filename,
		SHA1: mod.SHA1,
		SHA512: mod.SHA512,
		URL: verified.DownloadURL,
		Side: s.sideForPath(mod.Path),
		Deployment: mod.Location,
		Repository: repository,
		Tag: tag,
		Asset: verified.Asset,
		SourcePaths: s.sourcesForSHA(mod.SHA512),
	}
	s.replaceCatalogArtifact(mod, entry, "github:"+repository)
	if err := s.persistCatalogMutation(); err != nil {
		return ModManagementResult{}, err
	}
	if err := s.refreshSingleManagedEntry(ctx, entry); err != nil {
		return ModManagementResult{
			Action: "assign_github",
			Path: request.Path,
			Management: "managed",
			Provider: "github",
			ProjectID: repository,
			Message: "GitHub source was verified and saved, but its update metadata refresh failed: " + err.Error(),
		}, nil
	}
	return ModManagementResult{
		Action: "assign_github",
		Path: request.Path,
		Management: "managed",
		Provider: "github",
		ProjectID: repository,
		Message: "GitHub source verified and accepted.",
	}, nil
}

func (s *Service) forgetMissingAcceptedEntry(path string) (ModManagementResult, error) {
	found := false
	next := make([]catalog.Entry, 0, len(s.state.Catalog.Managed))
	for _, entry := range s.state.Catalog.Managed {
		sources := entry.SourcePaths[:0]
		for _, source := range entry.SourcePaths {
			if normalizeCatalogPath(source.Path) == path {
				found = true
				continue
			}
			sources = append(sources, source)
		}
		entry.SourcePaths = sources
		if len(entry.SourcePaths) > 0 {
			next = append(next, entry)
		}
	}
	if !found {
		return ModManagementResult{}, fmt.Errorf("no accepted managed entry uses path %q", path)
	}
	s.state.Catalog.Managed = next
	if err := s.persistCatalogMutation(); err != nil {
		return ModManagementResult{}, err
	}
	return ModManagementResult{
		Action: "forget_missing",
		Path: path,
		Message: "Missing accepted deployment entry was removed from FPBPack state.",
	}, nil
}

func (s *Service) refreshSingleManagedEntry(ctx context.Context, entry catalog.Entry) error {
	key := entry.Provider + ":" + entry.ProjectID
	report := updatecheck.Discover(ctx, catalog.Report{Managed: []catalog.Entry{entry}}, updatecheck.Options{
		Minecraft: s.options.Minecraft,
		Loader: s.options.Loader,
		Mode: updatecheck.RefreshModeInteractive,
		ModrinthBaseURL: s.options.ModrinthBaseURL,
		CurseForgeBaseURL: s.options.CurseForgeBaseURL,
		CurseForgeAPIKey: func() string {
			key, _ := s.effectiveCurseForgeAPIKey()
			return key
		}(),
		GitHubBaseURL: s.options.GitHubBaseURL,
		GitHubToken: s.options.GitHubToken,
	})
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(report.Candidates) != 1 {
		return fmt.Errorf("provider did not return exactly one metadata candidate")
	}

	s.mu.RLock()
	current := s.updates
	hasCurrent := s.hasUpdate
	s.mu.RUnlock()
	if hasCurrent {
		previous := updatecheck.Report{}
		for _, candidate := range current.Candidates {
			if candidate.Key == key {
				previous.Candidates = append(previous.Candidates, candidate)
				break
			}
		}
		if len(previous.Candidates) > 0 {
			preserveFailedMetadata(previous, &report)
		}
	}

	candidate := report.Candidates[0]
	if hasCurrent {
		replaced := false
		for index := range current.Candidates {
			if current.Candidates[index].Key == key {
				current.Candidates[index] = candidate
				replaced = true
				break
			}
		}
		if !replaced {
			current.Candidates = append(current.Candidates, candidate)
		}
		current.GeneratedAt = time.Now().UTC()
		current.Minecraft = s.options.Minecraft
		current.Loader = s.options.Loader
		current.RecalculateSummary()
		report = current
	}
	s.applyUpdateRules(&report)
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), report); err != nil {
		return fmt.Errorf("write update cache: %w", err)
	}
	s.mu.Lock()
	s.updates = report
	s.hasUpdate = true
	s.mu.Unlock()
	return nil
}

func (s *Service) persistCatalogMutation() error {
	s.state.Catalog.RecalculateSummary()
	s.state.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "state.json"), s.state); err != nil {
		return fmt.Errorf("persist management state: %w", err)
	}
	s.snapshot = management.BuildSnapshot(s.snapshot.Inventory, s.state.Catalog)
	s.pruneUpdateCandidatesToCatalog()
	return nil
}

func (s *Service) pruneUpdateCandidatesToCatalog() {
	managed := make(map[string]struct{}, len(s.state.Catalog.Managed))
	for _, entry := range s.state.Catalog.Managed {
		managed[entry.Provider+":"+entry.ProjectID] = struct{}{}
	}
	filtered := s.updates.Candidates[:0]
	for _, candidate := range s.updates.Candidates {
		if _, ok := managed[candidate.Key]; ok {
			filtered = append(filtered, candidate)
		}
	}
	s.updates.Candidates = filtered
	s.updates.RecalculateSummary()
	if s.hasUpdate {
		_ = writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), s.updates)
	}
}

func (s *Service) replaceCatalogArtifact(mod inventory.ModFile, entry catalog.Entry, managedKey string) {
	s.removeCatalogArtifact(mod, managedKey)
	s.state.Catalog.Managed = append(s.state.Catalog.Managed, entry)
	sort.Slice(s.state.Catalog.Managed, func(i, j int) bool {
		if s.state.Catalog.Managed[i].Name != s.state.Catalog.Managed[j].Name {
			return strings.ToLower(s.state.Catalog.Managed[i].Name) < strings.ToLower(s.state.Catalog.Managed[j].Name)
		}
		return s.state.Catalog.Managed[i].ProjectID < s.state.Catalog.Managed[j].ProjectID
	})
}

func (s *Service) removeCatalogArtifact(mod inventory.ModFile, managedKey string) {
	path := normalizeCatalogPath(mod.Path)
	s.state.Catalog.Unresolved = filterUnresolved(s.state.Catalog.Unresolved, path, mod.SHA512)
	s.state.Catalog.Pinned = filterPinned(s.state.Catalog.Pinned, path, mod.SHA512)

	managed := make([]catalog.Entry, 0, len(s.state.Catalog.Managed))
	for _, entry := range s.state.Catalog.Managed {
		entryKey := entry.Provider + ":" + entry.ProjectID
		remove := entry.SHA512 == mod.SHA512 || sourcesContainPath(entry.SourcePaths, path)
		if managedKey != "" && entryKey == managedKey {
			remove = true
		}
		if !remove {
			managed = append(managed, entry)
		}
	}
	s.state.Catalog.Managed = managed

	conflicts := make([]catalog.Conflict, 0, len(s.state.Catalog.Conflicts))
	for _, conflict := range s.state.Catalog.Conflicts {
		files := conflict.Files[:0]
		for _, file := range conflict.Files {
			if file.SHA512 == mod.SHA512 || sourcesContainPath(file.Sources, path) {
				continue
			}
			files = append(files, file)
		}
		if len(files) > 0 {
			conflict.Files = files
			conflicts = append(conflicts, conflict)
		}
	}
	s.state.Catalog.Conflicts = conflicts
}

func (s *Service) liveModByPath(path string) (inventory.ModFile, bool) {
	path = normalizeCatalogPath(path)
	for _, mod := range s.snapshot.Inventory.Mods {
		if normalizeCatalogPath(mod.Path) == path {
			return mod, true
		}
	}
	return inventory.ModFile{}, false
}

func (s *Service) managedEntryByPath(path string) (catalog.Entry, bool) {
	path = normalizeCatalogPath(path)
	for _, entry := range s.state.Catalog.Managed {
		if sourcesContainPath(entry.SourcePaths, path) {
			return entry, true
		}
	}
	return catalog.Entry{}, false
}

func (s *Service) isPinnedArtifact(mod inventory.ModFile) bool {
	for _, item := range s.state.Catalog.Pinned {
		if item.SHA512 == mod.SHA512 || sourcesContainPath(item.Sources, mod.Path) {
			return true
		}
	}
	return false
}

func (s *Service) sourcesForSHA(sha512Value string) []catalog.Source {
	result := make([]catalog.Source, 0)
	for _, mod := range s.snapshot.Inventory.Mods {
		if mod.SHA512 == sha512Value {
			result = append(result, catalog.Source{Location: mod.Location, Path: mod.Path})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Location != result[j].Location {
			return result[i].Location < result[j].Location
		}
		return result[i].Path < result[j].Path
	})
	return result
}

func (s *Service) sideForPath(path string) string {
	for _, mod := range s.snapshot.Mods {
		if normalizeCatalogPath(mod.Path) == normalizeCatalogPath(path) && mod.Side != "" {
			return mod.Side
		}
	}
	if live, ok := s.liveModByPath(path); ok && live.Location == inventory.LocationClient {
		return "client"
	}
	return "both"
}

func filterUnresolved(items []catalog.Unresolved, path, sha512Value string) []catalog.Unresolved {
	result := items[:0]
	for _, item := range items {
		if item.SHA512 == sha512Value || sourcesContainPath(item.Sources, path) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func filterPinned(items []catalog.PinnedArtifact, path, sha512Value string) []catalog.PinnedArtifact {
	result := items[:0]
	for _, item := range items {
		if item.SHA512 == sha512Value || sourcesContainPath(item.Sources, path) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func sourcesContainPath(sources []catalog.Source, path string) bool {
	path = normalizeCatalogPath(path)
	for _, source := range sources {
		if normalizeCatalogPath(source.Path) == path {
			return true
		}
	}
	return false
}

func normalizeCatalogPath(path string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(strings.TrimSpace(path))), "./")
}

func managementDisplayName(mod inventory.ModFile) string {
	for _, metadata := range mod.Metadata {
		if strings.TrimSpace(metadata.Name) != "" {
			return metadata.Name
		}
	}
	return mod.Filename
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
