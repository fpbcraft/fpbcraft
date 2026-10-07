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
	Placement  string `json:"placement,omitempty"`
	AutoModpackGroup string `json:"automodpack_group,omitempty"`
	ReplacesPath string `json:"replaces_path,omitempty"`
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
	request.ReplacesPath = normalizeCatalogPath(request.ReplacesPath)
	request.AutoModpackGroup = strings.TrimSpace(request.AutoModpackGroup)
	if request.Path == "" {
		return ModManagementResult{}, fmt.Errorf("mod path is required")
	}

	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	switch request.Action {
	case "mark_unmanaged":
		s.mu.Lock()
		result, err := s.markModUnmanaged(request.Path)
		s.mu.Unlock()
		s.logModManagement(request.Action, result, err)
		return result, err
	case "forget_missing":
		s.mu.Lock()
		result, err := s.forgetMissingAcceptedEntry(request.Path)
		s.mu.Unlock()
		s.logModManagement(request.Action, result, err)
		return result, err
	case "set_placement":
		s.mu.Lock()
		result, err := s.setPreferredPlacement(request.Path, request.Placement, request.AutoModpackGroup)
		s.mu.Unlock()
		s.logModManagement(request.Action, result, err)
		return result, err
	case "assign_modrinth", "assign_curseforge", "assign_github", "adopt_current":
		if !s.refreshMu.TryLock() {
			return ModManagementResult{}, fmt.Errorf(
				"provider refresh is in progress; source assignment would race with provider metadata reconciliation, so retry after it finishes",
			)
		}
		defer s.refreshMu.Unlock()
		var result ModManagementResult
		var err error
		switch request.Action {
		case "assign_modrinth":
			result, err = s.assignModrinthSource(ctx, request)
		case "assign_curseforge":
			result, err = s.assignCurseForgeSource(ctx, request)
		case "assign_github":
			result, err = s.assignGitHubSource(ctx, request)
		default:
			result, err = s.adoptCurrentArtifact(ctx, request)
		}
		s.logModManagement(request.Action, result, err)
		return result, err
	default:
		err := fmt.Errorf("unsupported mod management action %q", request.Action)
		s.logModManagement(request.Action, ModManagementResult{}, err)
		return ModManagementResult{}, err
	}
}

func (s *Service) RefreshModMetadata(ctx context.Context, path string) (err error) {
	path = normalizeCatalogPath(path)
	if path == "" {
		return fmt.Errorf("mod path is required")
	}

	if !s.refreshMu.TryLock() {
		return fmt.Errorf("another refresh is already in progress")
	}
	defer s.refreshMu.Unlock()
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()
	s.beginRefresh("mod", "Refreshing metadata for "+path)
	defer func() { s.finishRefresh(err) }()
	s.setRefreshProgress("providers", "Refreshing metadata for "+path, 0, 1, 10)

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
	s.mu.Lock()
	s.replaceCatalogArtifact(mod, entry)
	if err := s.persistCatalogMutation(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	return s.refreshSingleManagedEntry(ctx, entry)
}

func (s *Service) adoptCurrentArtifact(ctx context.Context, request ModManagementRequest) (ModManagementResult, error) {
	path := request.Path
	mod, ok := s.liveModByPath(path)
	if !ok {
		return ModManagementResult{}, fmt.Errorf(
			"live mod %q is not in the cached inventory; run an inventory refresh first",
			path,
		)
	}

	previous, ok := s.managedEntryByPath(path)
	if !ok && request.ReplacesPath != "" {
		candidate, found := s.managedEntryByPath(request.ReplacesPath)
		if !found {
			return ModManagementResult{}, fmt.Errorf(
				"accepted managed artifact %q was not found",
				request.ReplacesPath,
			)
		}
		for _, source := range candidate.SourcePaths {
			if _, exists := s.liveModByPath(source.Path); exists {
				return ModManagementResult{}, fmt.Errorf(
					"cannot replace %q because its accepted live artifact still exists at %s",
					request.ReplacesPath,
					source.Path,
				)
			}
		}
		previous = candidate
		ok = true
	}
	if !ok && mod.Modrinth != nil {
		for _, entry := range s.state.Catalog.Managed {
			if entry.Provider != "modrinth" || entry.ProjectID != mod.Modrinth.ProjectID {
				continue
			}
			liveSource := false
			for _, source := range entry.SourcePaths {
				if _, exists := s.liveModByPath(source.Path); exists {
					liveSource = true
					break
				}
			}
			if !liveSource {
				if ok {
					return ModManagementResult{}, fmt.Errorf(
						"multiple missing accepted Modrinth entries match project %s; choose the source explicitly",
						mod.Modrinth.ProjectID,
					)
				}
				previous = entry
				ok = true
			}
		}
	}
	if !ok {
		return ModManagementResult{}, fmt.Errorf(
			"FPBPack cannot identify which accepted managed artifact this JAR replaces; refresh inventory and use explicit source assignment if necessary",
		)
	}

	entry := previous
	if name := metadataDisplayName(mod); name != "" {
		entry.Name = name
	}
	entry.Filename = mod.Filename
	entry.SHA1 = mod.SHA1
	entry.SHA512 = mod.SHA512
	entry.SourcePaths = s.sourcesForSHA(mod.SHA512)
	if len(entry.SourcePaths) == 0 {
		entry.SourcePaths = []catalog.Source{{Location: mod.Location, Group: mod.Group, Path: mod.Path}}
	}

	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	switch previous.Provider {
	case "modrinth":
		match := mod.Modrinth
		if match == nil {
			matches, err := (inventory.ModrinthClient{BaseURL: s.options.ModrinthBaseURL}).Match(
				verifyCtx,
				[]inventory.ModFile{mod},
			)
			if err != nil {
				return ModManagementResult{}, fmt.Errorf("verify current Modrinth artifact: %w", err)
			}
			value, found := matches[mod.SHA512]
			if !found {
				return ModManagementResult{}, fmt.Errorf("current JAR has no exact Modrinth hash match")
			}
			match = &value
		}
		if match.ProjectID != previous.ProjectID {
			return ModManagementResult{}, fmt.Errorf(
				"current JAR belongs to Modrinth project %s, but the accepted artifact belongs to %s",
				match.ProjectID,
				previous.ProjectID,
			)
		}
		entry.VersionID = match.VersionID
		entry.URL = match.URL
		entry.Environment = match.Environment
	case "curseforge":
		apiKey, _ := s.effectiveCurseForgeAPIKey()
		if apiKey == "" {
			return ModManagementResult{}, fmt.Errorf(
				"configure a CurseForge API key before adopting this manually replaced JAR",
			)
		}
		verified, err := (&updatecheck.CurseForgeClient{
			BaseURL: s.options.CurseForgeBaseURL,
			APIKey: apiKey,
			Mode: updatecheck.RefreshModeInteractive,
		}).ResolveInstalledFile(
			verifyCtx,
			previous.ProjectID,
			s.options.Minecraft,
			s.options.Loader,
			mod.SHA1,
		)
		if err != nil {
			return ModManagementResult{}, fmt.Errorf("verify current CurseForge artifact: %w", err)
		}
		entry.FileID = verified.FileID
	case "github":
		livePath := filepath.Join(s.options.ServerRoot, filepath.FromSlash(mod.Path))
		sha256Value, err := sha256File(livePath)
		if err != nil {
			return ModManagementResult{}, fmt.Errorf("hash current JAR: %w", err)
		}
		repository := previous.Repository
		if strings.TrimSpace(repository) == "" {
			repository = previous.ProjectID
		}
		verified, err := (&updatecheck.GitHubClient{
			BaseURL: s.options.GitHubBaseURL,
			Token: s.options.GitHubToken,
			Mode: updatecheck.RefreshModeInteractive,
		}).ResolveInstalledAsset(verifyCtx, repository, sha256Value)
		if err != nil {
			return ModManagementResult{}, fmt.Errorf("verify current GitHub artifact: %w", err)
		}
		entry.ProjectID = repository
		entry.Repository = repository
		entry.VersionID = verified.Tag
		entry.Tag = verified.Tag
		entry.Asset = verified.Asset
		entry.URL = verified.DownloadURL
	default:
		return ModManagementResult{}, fmt.Errorf(
			"adopting manually replaced %s artifacts is not supported",
			previous.Provider,
		)
	}

	s.mu.Lock()
	previousKey := catalog.EntryKey(previous)
	s.removeCatalogArtifact(mod)
	managed := make([]catalog.Entry, 0, len(s.state.Catalog.Managed))
	for _, candidate := range s.state.Catalog.Managed {
		if catalog.EntryKey(candidate) == previousKey {
			continue
		}
		managed = append(managed, candidate)
	}
	s.state.Catalog.Managed = managed
	entry.ArtifactID = previous.ArtifactID
	entry.Deployment = previous.Deployment
	entry.AutoModpackGroup = previous.AutoModpackGroup
	s.state.Catalog.Managed = append(s.state.Catalog.Managed, entry)
	catalog.EnsureManagedArtifactIDs(s.state.Catalog.Managed)
	sort.Slice(s.state.Catalog.Managed, func(i, j int) bool {
		return strings.ToLower(s.state.Catalog.Managed[i].Name) <
			strings.ToLower(s.state.Catalog.Managed[j].Name)
	})
	if err := s.persistCatalogMutation(); err != nil {
		s.mu.Unlock()
		return ModManagementResult{}, err
	}
	s.mu.Unlock()

	message := fmt.Sprintf(
		"Current JAR verified as %s project %s and adopted into accepted state.",
		entry.Provider,
		entry.ProjectID,
	)
	if err := s.refreshSingleManagedEntry(ctx, entry); err != nil {
		message += " Update metadata refresh failed: " + err.Error()
	}
	return ModManagementResult{
		Action: "adopt_current",
		Path: mod.Path,
		Management: "managed",
		Provider: entry.Provider,
		ProjectID: entry.ProjectID,
		Message: message,
	}, nil
}

func (s *Service) markModUnmanaged(path string) (ModManagementResult, error) {
	mod, ok := s.liveModByPath(path)
	if !ok {
		return ModManagementResult{}, fmt.Errorf("live mod %q was not found", path)
	}

	s.removeCatalogArtifact(mod)
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
	s.mu.Lock()
	s.replaceCatalogArtifact(mod, entry)
	if err := s.persistCatalogMutation(); err != nil {
		s.mu.Unlock()
		return ModManagementResult{}, err
	}
	s.mu.Unlock()
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
	if _, err := client.VerifyInstalledFile(
		verifyCtx,
		projectID,
		request.FileID,
		mod.SHA1,
	); err != nil {
		return ModManagementResult{}, fmt.Errorf("verify CurseForge source: %w", err)
	}

	name := managementDisplayName(mod)
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
	s.mu.Lock()
	s.replaceCatalogArtifact(mod, entry)
	if err := s.persistCatalogMutation(); err != nil {
		s.mu.Unlock()
		return ModManagementResult{}, err
	}
	s.mu.Unlock()
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
	repository = strings.TrimPrefix(repository, "https://github.com/")
	repository = strings.TrimPrefix(repository, "http://github.com/")
	repository = strings.TrimRight(repository, "/")
	tag := strings.TrimSpace(request.Tag)
	asset := strings.TrimSpace(request.Asset)
	if asset == "" {
		asset = mod.Filename
	}
	if repository == "" || tag == "" {
		return ModManagementResult{}, fmt.Errorf("GitHub repository and installed release tag are required")
	}
	if strings.Count(repository, "/") != 1 {
		return ModManagementResult{}, fmt.Errorf("GitHub repository must be in owner/repository form")
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
	s.mu.Lock()
	s.replaceCatalogArtifact(mod, entry)
	if err := s.persistCatalogMutation(); err != nil {
		s.mu.Unlock()
		return ModManagementResult{}, err
	}
	s.mu.Unlock()
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
	relative, err := safeRelativePath(path)
	if err != nil {
		return ModManagementResult{}, err
	}
	livePath := filepath.Join(s.options.ServerRoot, relative)
	if _, err := os.Stat(livePath); err == nil {
		return ModManagementResult{}, fmt.Errorf(
			"cannot forget %q because the live file still exists; refresh inventory or manage the installed artifact instead",
			path,
		)
	} else if !os.IsNotExist(err) {
		return ModManagementResult{}, fmt.Errorf("check live artifact before forgetting it: %w", err)
	}

	found := false
	removedFilenames := map[string]struct{}{}

	nextManaged := make([]catalog.Entry, 0, len(s.state.Catalog.Managed))
	for _, entry := range s.state.Catalog.Managed {
		sources := make([]catalog.Source, 0, len(entry.SourcePaths))
		removed := false
		for _, source := range entry.SourcePaths {
			if normalizeCatalogPath(source.Path) == path {
				found = true
				removed = true
				continue
			}
			sources = append(sources, source)
		}
		entry.SourcePaths = sources
		if removed {
			removedFilenames[entry.Filename] = struct{}{}
		}
		if len(entry.SourcePaths) > 0 {
			nextManaged = append(nextManaged, entry)
		}
	}
	s.state.Catalog.Managed = nextManaged

	nextUnresolved := make([]catalog.Unresolved, 0, len(s.state.Catalog.Unresolved))
	for _, item := range s.state.Catalog.Unresolved {
		sources := make([]catalog.Source, 0, len(item.Sources))
		removed := false
		for _, source := range item.Sources {
			if normalizeCatalogPath(source.Path) == path {
				found = true
				removed = true
				continue
			}
			sources = append(sources, source)
		}
		item.Sources = sources
		if removed {
			removedFilenames[item.Filename] = struct{}{}
		}
		if len(item.Sources) > 0 {
			nextUnresolved = append(nextUnresolved, item)
		}
	}
	s.state.Catalog.Unresolved = nextUnresolved

	// Duplicate records are derived catalog metadata. If a forgotten path was
	// one side of a former duplicate, remove that stale source as well.
	nextDuplicates := make([]catalog.Duplicate, 0, len(s.state.Catalog.Duplicates))
	for _, duplicate := range s.state.Catalog.Duplicates {
		files := make([]catalog.Source, 0, len(duplicate.Files))
		for _, source := range duplicate.Files {
			if normalizeCatalogPath(source.Path) == path {
				continue
			}
			files = append(files, source)
		}
		duplicate.Files = files
		if len(duplicate.Files) > 1 {
			nextDuplicates = append(nextDuplicates, duplicate)
		}
	}
	s.state.Catalog.Duplicates = nextDuplicates

	if !found {
		return ModManagementResult{}, fmt.Errorf("no absent catalog entry uses path %q", path)
	}

	// Placement warnings are meaningful only while the referenced managed
	// artifact is still represented in the accepted catalog.
	if len(removedFilenames) > 0 {
		remaining := make([]catalog.PlacementWarning, 0, len(s.state.Catalog.Placement))
		for _, warning := range s.state.Catalog.Placement {
			if _, removed := removedFilenames[warning.Filename]; removed {
				continue
			}
			remaining = append(remaining, warning)
		}
		s.state.Catalog.Placement = remaining
	}

	if err := s.persistCatalogMutation(); err != nil {
		return ModManagementResult{}, err
	}
	return ModManagementResult{
		Action: "forget_missing",
		Path: path,
		Message: "Absent catalog entry was removed from FPBPack state. No server files were changed.",
	}, nil
}

func (s *Service) setPreferredPlacement(path, placement string, autoModpackGroups ...string) (ModManagementResult, error) {
	placement = strings.TrimSpace(strings.ToLower(placement))
	autoModpackGroup := ""
	if len(autoModpackGroups) > 0 {
		autoModpackGroup = autoModpackGroups[0]
	}
	var target inventory.Location
	switch placement {
	case string(inventory.LocationServer):
		target = inventory.LocationServer
	case string(inventory.LocationClient):
		target = inventory.LocationClient
	default:
		return ModManagementResult{}, fmt.Errorf("placement must be %q or %q", inventory.LocationServer, inventory.LocationClient)
	}
	group := normalizeAutoModpackGroup(target, autoModpackGroup)
	if target == inventory.LocationClient {
		if err := validateAutoModpackGroupID(group); err != nil {
			return ModManagementResult{}, err
		}
	}

	path = normalizeCatalogPath(path)
	updated := false
	for index := range s.state.Catalog.Managed {
		if sourcesContainPath(s.state.Catalog.Managed[index].SourcePaths, path) {
			s.state.Catalog.Managed[index].Deployment = target
			s.state.Catalog.Managed[index].AutoModpackGroup = group
			updated = true
			break
		}
	}
	if !updated {
		return ModManagementResult{}, fmt.Errorf("preferred placement can only be changed after the artifact has a verified management source")
	}
	if err := s.persistCatalogMutation(); err != nil {
		return ModManagementResult{}, err
	}
	destination := string(target)
	if target == inventory.LocationClient {
		destination = "AutoModpack/" + group
	}
	return ModManagementResult{
		Action: "set_placement",
		Path: path,
		Management: "managed",
		Message: fmt.Sprintf("Preferred placement set to %s. The live JAR is not moved until a protected Apply operation.", destination),
	}, nil
}

func (s *Service) refreshSingleManagedEntry(ctx context.Context, entry catalog.Entry) error {
	key := catalog.EntryKey(entry)
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
	catalog.EnsureManagedArtifactIDs(s.state.Catalog.Managed)
	s.state.Catalog.RecalculateSummary()
	s.state.UpdatedAt = time.Now().UTC()
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "state.json"), s.state); err != nil {
		return fmt.Errorf("persist management state: %w", err)
	}
	s.snapshot = management.BuildSnapshot(s.snapshot.Inventory, s.state.Catalog)
	if s.hasUpdate {
		reconcileUpdateReportToCatalog(&s.updates, s.state.Catalog)
		_ = writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), s.updates)
	} else {
		s.pruneUpdateCandidatesToCatalog()
	}
	return nil
}

func (s *Service) pruneUpdateCandidatesToCatalog() {
	managed := make(map[string]struct{}, len(s.state.Catalog.Managed))
	for _, entry := range s.state.Catalog.Managed {
		managed[catalog.EntryKey(entry)] = struct{}{}
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

func (s *Service) replaceCatalogArtifact(mod inventory.ModFile, entry catalog.Entry) {
	if previous, ok := s.managedEntryByPath(mod.Path); ok {
		// Carry the stable artifact discriminator and user's preferred placement
		// across source reassignment or installed-version verification.
		entry.ArtifactID = previous.ArtifactID
		entry.Deployment = previous.Deployment
		entry.AutoModpackGroup = previous.AutoModpackGroup
	} else {
		entry.AutoModpackGroup = normalizeAutoModpackGroup(entry.Deployment, mod.Group)
	}
	s.removeCatalogArtifact(mod)
	s.state.Catalog.Managed = append(s.state.Catalog.Managed, entry)
	catalog.EnsureManagedArtifactIDs(s.state.Catalog.Managed)
	sort.Slice(s.state.Catalog.Managed, func(i, j int) bool {
		if s.state.Catalog.Managed[i].Name != s.state.Catalog.Managed[j].Name {
			return strings.ToLower(s.state.Catalog.Managed[i].Name) < strings.ToLower(s.state.Catalog.Managed[j].Name)
		}
		if s.state.Catalog.Managed[i].ProjectID != s.state.Catalog.Managed[j].ProjectID {
			return s.state.Catalog.Managed[i].ProjectID < s.state.Catalog.Managed[j].ProjectID
		}
		return catalog.EntryKey(s.state.Catalog.Managed[i]) < catalog.EntryKey(s.state.Catalog.Managed[j])
	})
}

func (s *Service) removeCatalogArtifact(mod inventory.ModFile) {
	path := normalizeCatalogPath(mod.Path)
	s.state.Catalog.Unresolved = filterUnresolved(s.state.Catalog.Unresolved, path, mod.SHA512)
	s.state.Catalog.Pinned = filterPinned(s.state.Catalog.Pinned, path, mod.SHA512)

	managed := make([]catalog.Entry, 0, len(s.state.Catalog.Managed))
	for _, entry := range s.state.Catalog.Managed {
		remove := entry.SHA512 == mod.SHA512 || sourcesContainPath(entry.SourcePaths, path)
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
			result = append(result, catalog.Source{Location: mod.Location, Group: mod.Group, Path: mod.Path})
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

func metadataDisplayName(mod inventory.ModFile) string {
	for _, metadata := range mod.Metadata {
		if name := strings.TrimSpace(metadata.Name); name != "" {
			return name
		}
	}
	return ""
}

func managementDisplayName(mod inventory.ModFile) string {
	if name := metadataDisplayName(mod); name != "" {
		return name
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
