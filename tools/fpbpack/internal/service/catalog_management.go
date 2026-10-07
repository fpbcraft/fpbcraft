package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

type CatalogPlanRequest struct {
	Action    string             `json:"action"`
	Provider  string             `json:"provider,omitempty"`
	ProjectID string             `json:"project_id,omitempty"`
	VersionID string             `json:"version_id,omitempty"`
	Path      string             `json:"path,omitempty"`
	Placement inventory.Location `json:"placement,omitempty"`
	AutoModpackGroup string       `json:"automodpack_group,omitempty"`
}

func (s *Service) SearchCatalog(
	ctx context.Context,
	provider string,
	query string,
) ([]updatecheck.CatalogProject, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	query = strings.TrimSpace(query)
	if query == "" {
		return []updatecheck.CatalogProject{}, nil
	}

	var (
		result []updatecheck.CatalogProject
		err    error
	)
	switch provider {
	case "modrinth":
		client := &updatecheck.ModrinthClient{
			BaseURL: s.options.ModrinthBaseURL,
			Mode:    updatecheck.RefreshModeInteractive,
		}
		result, err = client.SearchCatalogProjects(ctx, query, s.options.Minecraft, s.options.Loader, 20)
	case "curseforge":
		key, source := s.effectiveCurseForgeAPIKey()
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("CurseForge API key is not configured")
		}
		client := &updatecheck.CurseForgeClient{
			BaseURL: s.options.CurseForgeBaseURL,
			APIKey:  key,
			Mode:    updatecheck.RefreshModeInteractive,
		}
		result, err = client.SearchCatalogProjects(ctx, query, s.options.Minecraft, s.options.Loader, 20)
		if err != nil && source != "" {
			return nil, fmt.Errorf("CurseForge search using %s credential: %w", source, err)
		}
	default:
		return nil, fmt.Errorf("unsupported catalog provider %q", provider)
	}
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	managed := append([]catalog.Entry(nil), s.state.Catalog.Managed...)
	s.mu.RUnlock()
	for index := range result {
		for _, entry := range managed {
			if entry.Provider == result[index].Provider && entry.ProjectID == result[index].ProjectID {
				result[index].Installed = true
				result[index].InstalledKey = catalog.EntryKey(entry)
				break
			}
		}
	}
	return result, nil
}

func (s *Service) CatalogVersions(
	ctx context.Context,
	provider string,
	projectID string,
) ([]updatecheck.CatalogVersion, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}
	switch provider {
	case "modrinth":
		client := &updatecheck.ModrinthClient{
			BaseURL: s.options.ModrinthBaseURL,
			Mode:    updatecheck.RefreshModeInteractive,
		}
		return client.CatalogVersions(ctx, projectID, s.options.Minecraft, s.options.Loader)
	case "curseforge":
		key, _ := s.effectiveCurseForgeAPIKey()
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("CurseForge API key is not configured")
		}
		client := &updatecheck.CurseForgeClient{
			BaseURL: s.options.CurseForgeBaseURL,
			APIKey:  key,
			Mode:    updatecheck.RefreshModeInteractive,
		}
		return client.CatalogVersions(ctx, projectID, s.options.Minecraft, s.options.Loader)
	default:
		return nil, fmt.Errorf("unsupported catalog provider %q", provider)
	}
}

func normalizeCatalogPlanRequest(request CatalogPlanRequest) CatalogPlanRequest {
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.Provider = strings.ToLower(strings.TrimSpace(request.Provider))
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.VersionID = strings.TrimSpace(request.VersionID)
	request.Path = normalizeCatalogPath(request.Path)
	request.AutoModpackGroup = strings.TrimSpace(request.AutoModpackGroup)
	return request
}

func (s *Service) catalogCandidateForPlanRequest(
	ctx context.Context,
	request CatalogPlanRequest,
	cat catalog.Report,
) (updatecheck.Candidate, CatalogPlanRequest, error) {
	request = normalizeCatalogPlanRequest(request)

	var candidate updatecheck.Candidate
	switch request.Action {
	case "install":
		if request.Provider != "modrinth" && request.Provider != "curseforge" {
			return updatecheck.Candidate{}, request, fmt.Errorf("install supports Modrinth or CurseForge projects")
		}
		if request.ProjectID == "" || request.VersionID == "" {
			return updatecheck.Candidate{}, request, fmt.Errorf("provider project and exact version are required")
		}
		for _, entry := range cat.Managed {
			if entry.Provider == request.Provider && entry.ProjectID == request.ProjectID {
				return updatecheck.Candidate{}, request, fmt.Errorf("%s project %s is already managed; use Change version instead", request.Provider, request.ProjectID)
			}
		}
		placement, err := normalizeCatalogPlacement(request.Placement, inventory.LocationServer)
		if err != nil {
			return updatecheck.Candidate{}, request, err
		}
		candidate, err = s.catalogCandidate(ctx, request.Provider, request.ProjectID, request.VersionID, placement, "install", nil, cat)
		if err != nil {
			return updatecheck.Candidate{}, request, err
		}
		request.Placement = placement
		candidate.AutoModpackGroup = normalizeAutoModpackGroup(placement, request.AutoModpackGroup)
		request.AutoModpackGroup = candidate.AutoModpackGroup
		inheritClientDependencyGroups(candidate.Dependencies, candidate.AutoModpackGroup)

	case "version":
		entry, ok := managedCatalogEntryByPath(cat, request.Path)
		if !ok {
			return updatecheck.Candidate{}, request, fmt.Errorf("managed artifact %q was not found", request.Path)
		}
		if entry.Provider != "modrinth" && entry.Provider != "curseforge" {
			return updatecheck.Candidate{}, request, fmt.Errorf("exact version browsing currently supports Modrinth and CurseForge managed artifacts")
		}
		if request.VersionID == "" {
			return updatecheck.Candidate{}, request, fmt.Errorf("exact version is required")
		}
		placement, err := normalizeCatalogPlacement(request.Placement, entry.Deployment)
		if err != nil {
			return updatecheck.Candidate{}, request, err
		}
		candidate, err = s.catalogCandidate(ctx, entry.Provider, entry.ProjectID, request.VersionID, placement, "version", &entry, cat)
		if err != nil {
			return updatecheck.Candidate{}, request, err
		}
		group := request.AutoModpackGroup
		if strings.TrimSpace(group) == "" {
			group = entry.AutoModpackGroup
		}
		request.Provider = entry.Provider
		request.ProjectID = entry.ProjectID
		request.Placement = placement
		candidate.AutoModpackGroup = normalizeAutoModpackGroup(placement, group)
		request.AutoModpackGroup = candidate.AutoModpackGroup
		inheritClientDependencyGroups(candidate.Dependencies, candidate.AutoModpackGroup)

	case "remove":
		entry, ok := managedCatalogEntryByPath(cat, request.Path)
		if !ok {
			return updatecheck.Candidate{}, request, fmt.Errorf("managed artifact %q was not found", request.Path)
		}
		if entry.Provider != "modrinth" && entry.Provider != "curseforge" {
			return updatecheck.Candidate{}, request, fmt.Errorf(
				"safe removal currently requires Modrinth or CurseForge dependency metadata",
			)
		}
		requiredBy, err := s.currentRequiredBy(ctx, entry, cat)
		if err != nil {
			return updatecheck.Candidate{}, request, err
		}
		versionID := entry.VersionID
		if entry.Provider == "curseforge" && entry.FileID != 0 {
			versionID = strconv.FormatUint(uint64(entry.FileID), 10)
		}
		request.Provider = entry.Provider
		request.ProjectID = entry.ProjectID
		request.Placement = entry.Deployment
		request.AutoModpackGroup = normalizeAutoModpackGroup(entry.Deployment, entry.AutoModpackGroup)
		candidate = updatecheck.Candidate{
			Key:              catalog.EntryKey(entry),
			Provider:         entry.Provider,
			ProjectID:        entry.ProjectID,
			Name:             entry.Name,
			Side:             entry.Side,
			Deployment:       entry.Deployment,
			AutoModpackGroup: request.AutoModpackGroup,
			Environment:      entry.Environment,
			Installed: updatecheck.Release{
				ID:       versionID,
				Number:   versionID,
				Name:     entry.Name,
				Filename: entry.Filename,
				URL:      entry.URL,
				SHA1:     entry.SHA1,
				SHA512:   entry.SHA512,
			},
			Classification: updatecheck.ClassificationReview,
			Intent:         "remove",
			RequiredBy:     requiredBy,
			Reasons: []updatecheck.Reason{{
				Code:    "catalog_remove",
				Message: "This managed mod was explicitly selected for removal.",
			}},
		}

	default:
		return updatecheck.Candidate{}, request, fmt.Errorf("unsupported catalog action %q", request.Action)
	}

	return candidate, request, nil
}

func (s *Service) CreateCatalogPlan(ctx context.Context, request CatalogPlanRequest) (planning.Plan, error) {
	if !s.refreshMu.TryLock() {
		return planning.Plan{}, fmt.Errorf("provider refresh is in progress; retry catalog planning after it finishes")
	}
	defer s.refreshMu.Unlock()
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	s.mu.RLock()
	snapshot := s.snapshot
	cat := s.state.Catalog
	s.mu.RUnlock()

	candidate, _, err := s.catalogCandidateForPlanRequest(ctx, request, cat)
	if err != nil {
		return planning.Plan{}, err
	}

	now := time.Now().UTC()
	exactReport := updatecheck.Report{
		GeneratedAt: now,
		Minecraft:   s.options.Minecraft,
		Loader:      s.options.Loader,
		Candidates:  []updatecheck.Candidate{candidate},
	}
	exactReport.RecalculateSummary()
	plan, err := planning.Build([]string{candidate.Key}, exactReport, snapshot, now)
	if err != nil {
		return planning.Plan{}, err
	}
	return s.persistPlannedChange(ctx, plan)
}

func (s *Service) catalogCandidate(
	ctx context.Context,
	provider string,
	projectID string,
	versionID string,
	deployment inventory.Location,
	intent string,
	current *catalog.Entry,
	cat catalog.Report,
) (updatecheck.Candidate, error) {
	opts := updatecheck.Options{
		Minecraft: s.options.Minecraft,
		Loader:    s.options.Loader,
		Mode:      updatecheck.RefreshModeInteractive,
	}
	switch provider {
	case "modrinth":
		client := &updatecheck.ModrinthClient{
			BaseURL: s.options.ModrinthBaseURL,
			Mode:    updatecheck.RefreshModeInteractive,
		}
		return client.CatalogCandidate(ctx, projectID, versionID, deployment, intent, current, cat, opts)
	case "curseforge":
		key, _ := s.effectiveCurseForgeAPIKey()
		if strings.TrimSpace(key) == "" {
			return updatecheck.Candidate{}, fmt.Errorf("CurseForge API key is not configured")
		}
		fileID, err := strconv.ParseUint(versionID, 10, 32)
		if err != nil || fileID == 0 {
			return updatecheck.Candidate{}, fmt.Errorf("invalid CurseForge file ID %q", versionID)
		}
		client := &updatecheck.CurseForgeClient{
			BaseURL: s.options.CurseForgeBaseURL,
			APIKey:  key,
			Mode:    updatecheck.RefreshModeInteractive,
		}
		return client.CatalogCandidate(ctx, projectID, uint32(fileID), deployment, intent, current, cat, opts)
	default:
		return updatecheck.Candidate{}, fmt.Errorf("unsupported catalog provider %q", provider)
	}
}

func managedCatalogEntryByPath(cat catalog.Report, path string) (catalog.Entry, bool) {
	path = normalizeCatalogPath(path)
	if path == "" {
		return catalog.Entry{}, false
	}
	for _, entry := range cat.Managed {
		if sourcesContainPath(entry.SourcePaths, path) {
			return entry, true
		}
	}
	return catalog.Entry{}, false
}

func normalizeCatalogPlacement(value inventory.Location, fallback inventory.Location) (inventory.Location, error) {
	if value == "" {
		value = fallback
	}
	switch value {
	case inventory.LocationServer, inventory.LocationClient:
		return value, nil
	default:
		return "", fmt.Errorf("placement must be %q or %q", inventory.LocationServer, inventory.LocationClient)
	}
}
