package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

// Check installed dependents too: a library upgrade can violate an exact provider
// requirement of a mod that was never selected for an update.
func (s *Service) validateProviderRequirements(ctx context.Context, plan *planning.Plan) {
	if plan.Status == planning.StatusBlocked {
		return
	}
	s.mu.RLock()
	current := s.state.Catalog
	s.mu.RUnlock()
	proposed, err := catalogAfterPlan(current, *plan)
	if err != nil {
		plan.Blockers = append(plan.Blockers, planning.Finding{Code: "proposed_catalog_invalid", Message: err.Error()})
		return
	}
	versions := []string{}
	for _, entry := range proposed.Managed {
		if entry.Provider == "modrinth" && entry.VersionID != "" {
			versions = append(versions, entry.VersionID)
		}
	}
	if len(versions) == 0 {
		return
	}
	client := &updatecheck.ModrinthClient{BaseURL: s.options.ModrinthBaseURL, Mode: updatecheck.RefreshModeInteractive}
	metadata, err := client.GetVersions(ctx, versions)
	if err != nil {
		plan.Warnings = append(plan.Warnings, planning.Finding{Code: "provider_dependency_unverified", Message: "Could not verify final installed Modrinth dependency metadata: " + err.Error()})
		return
	}
	byProject := map[string][]catalog.Entry{}
	for _, entry := range proposed.Managed {
		byProject[entry.Provider+":"+entry.ProjectID] = append(byProject[entry.Provider+":"+entry.ProjectID], entry)
	}
	// Include exact cross-provider identities only for bytes which remain unchanged.
	s.mu.RLock()
	files := s.snapshot.Inventory.Mods
	s.mu.RUnlock()
	for _, file := range files {
		if file.Modrinth == nil {
			continue
		}
		for _, entry := range proposed.Managed {
			if entry.Provider != "modrinth" && strings.EqualFold(file.SHA512, entry.SHA512) {
				alias := entry
				alias.VersionID = file.Modrinth.VersionID
				byProject["modrinth:"+file.Modrinth.ProjectID] = append(byProject["modrinth:"+file.Modrinth.ProjectID], alias)
			}
		}
	}
	extra := []string{}
	for _, version := range metadata {
		for _, dep := range version.Dependencies {
			if dep.ProjectID == "" && dep.VersionID != "" && dep.DependencyType == "required" {
				extra = append(extra, dep.VersionID)
			}
		}
	}
	if len(extra) > 0 {
		resolved, resolveErr := client.GetVersions(ctx, extra)
		if resolveErr != nil {
			plan.Warnings = append(plan.Warnings, planning.Finding{Code: "provider_dependency_unverified", Message: resolveErr.Error()})
		} else {
			for id, version := range resolved {
				metadata[id] = version
			}
		}
	}
	for _, owner := range proposed.Managed {
		if owner.Provider != "modrinth" {
			continue
		}
		version, ok := metadata[owner.VersionID]
		if !ok {
			plan.Warnings = append(plan.Warnings, planning.Finding{Code: "provider_dependency_unverified", Name: owner.Name, Message: "Installed dependency metadata was not returned for " + owner.Name})
			continue
		}
		for _, dep := range version.Dependencies {
			if dep.DependencyType != "required" && dep.DependencyType != "incompatible" {
				continue
			}
			project := dep.ProjectID
			if project == "" && dep.VersionID != "" {
				project = metadata[dep.VersionID].ProjectID
			}
			if project == "" {
				plan.Warnings = append(plan.Warnings, planning.Finding{Code: "provider_dependency_unverified", Name: owner.Name, Message: "Could not identify dependency " + dep.VersionID + " for " + owner.Name})
				continue
			}
			satisfied, conflict, uncertain := providerDependencyAvailability(owner, byProject["modrinth:"+project], dep.VersionID)
			if uncertain {
				plan.Warnings = append(plan.Warnings, planning.Finding{Code: "provider_dependency_delivery_unverified", Name: owner.Name, Message: fmt.Sprintf("Verify AutoModpack delivery of %s %s for %s; only a server/common copy is known for at least one client location.", project, dep.VersionID, owner.Name)})
			}

			if (dep.DependencyType == "required" && !satisfied) || (dep.DependencyType == "incompatible" && conflict) {
				path := ""
				if len(owner.SourcePaths) > 0 {
					path = owner.SourcePaths[0].Path
				}
				plan.Blockers = append(plan.Blockers, planning.Finding{Code: "final_provider_dependency_conflict", CandidateKey: catalog.EntryKey(owner), Name: owner.Name, Path: path, Message: fmt.Sprintf("%s declares %s dependency %s %s; the complete proposed catalog does not satisfy it.", owner.Name, dep.DependencyType, project, dep.VersionID)})
			}
		}
	}
}
