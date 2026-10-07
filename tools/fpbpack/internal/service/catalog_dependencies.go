package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func (s *Service) currentRequiredBy(
	ctx context.Context,
	target catalog.Entry,
	cat catalog.Report,
) ([]string, error) {
	names := make([]string, 0)
	seen := map[string]struct{}{}
	appendName := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	switch target.Provider {
	case "modrinth":
		client := &updatecheck.ModrinthClient{
			BaseURL: s.options.ModrinthBaseURL,
			Mode:    updatecheck.RefreshModeInteractive,
		}

		entries := make([]catalog.Entry, 0, len(cat.Managed))
		versionIDs := make([]string, 0, len(cat.Managed))
		for _, entry := range cat.Managed {
			if entry.Provider != "modrinth" || entry.ProjectID == target.ProjectID {
				continue
			}
			versionID := strings.TrimSpace(entry.VersionID)
			if versionID == "" {
				return nil, fmt.Errorf(
					"cannot verify whether %s depends on %s because its installed Modrinth version ID is missing",
					entry.Name,
					target.Name,
				)
			}
			entries = append(entries, entry)
			versionIDs = append(versionIDs, versionID)
		}

		versions, err := client.GetVersions(ctx, versionIDs)
		if err != nil {
			return nil, fmt.Errorf("verify installed Modrinth dependency metadata: %w", err)
		}

		unresolvedByVersion := map[string][]string{}
		unresolvedIDs := make([]string, 0)
		for _, entry := range entries {
			versionID := strings.TrimSpace(entry.VersionID)
			version, ok := versions[versionID]
			if !ok {
				return nil, fmt.Errorf(
					"cannot verify whether %s depends on %s because Modrinth did not return installed version %s",
					entry.Name,
					target.Name,
					versionID,
				)
			}
			for _, dependency := range version.Dependencies {
				if dependency.DependencyType != "required" {
					continue
				}
				projectID := strings.TrimSpace(dependency.ProjectID)
				if projectID == target.ProjectID {
					appendName(entry.Name)
					break
				}
				dependencyVersionID := strings.TrimSpace(dependency.VersionID)
				if projectID != "" || dependencyVersionID == "" {
					continue
				}
				if _, exists := unresolvedByVersion[dependencyVersionID]; !exists {
					unresolvedIDs = append(unresolvedIDs, dependencyVersionID)
				}
				unresolvedByVersion[dependencyVersionID] = append(
					unresolvedByVersion[dependencyVersionID],
					entry.Name,
				)
			}
		}

		if len(unresolvedIDs) > 0 {
			dependencyVersions, err := client.GetVersions(ctx, unresolvedIDs)
			if err != nil {
				return nil, fmt.Errorf("resolve installed Modrinth dependency metadata: %w", err)
			}
			for _, dependencyVersionID := range unresolvedIDs {
				dependencyVersion, ok := dependencyVersions[dependencyVersionID]
				if !ok {
					return nil, fmt.Errorf(
						"cannot resolve installed dependency version %s from Modrinth",
						dependencyVersionID,
					)
				}
				if strings.TrimSpace(dependencyVersion.ProjectID) != target.ProjectID {
					continue
				}
				for _, entryName := range unresolvedByVersion[dependencyVersionID] {
					appendName(entryName)
				}
			}
		}

	case "curseforge":
		key, _ := s.effectiveCurseForgeAPIKey()
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf(
				"CurseForge API key is required to verify reverse dependencies before removal",
			)
		}
		client := &updatecheck.CurseForgeClient{
			BaseURL: s.options.CurseForgeBaseURL,
			APIKey:  key,
			Mode:    updatecheck.RefreshModeInteractive,
		}
		targetID, err := strconv.Atoi(target.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("invalid CurseForge project ID %q", target.ProjectID)
		}
		for _, entry := range cat.Managed {
			if entry.Provider != "curseforge" || entry.ProjectID == target.ProjectID {
				continue
			}
			if entry.FileID == 0 {
				return nil, fmt.Errorf(
					"cannot verify whether %s depends on %s because its installed CurseForge file ID is missing",
					entry.Name,
					target.Name,
				)
			}
			file, err := client.GetFile(ctx, entry.ProjectID, entry.FileID)
			if err != nil {
				return nil, fmt.Errorf(
					"verify installed dependency metadata for %s: %w",
					entry.Name,
					err,
				)
			}
			for _, dependency := range file.Dependencies {
				if dependency.RelationType == 3 && dependency.ModID == targetID {
					appendName(entry.Name)
					break
				}
			}
		}

	default:
		return nil, fmt.Errorf(
			"provider %q does not expose installed dependency metadata for safe removal",
			target.Provider,
		)
	}

	return names, nil
}
