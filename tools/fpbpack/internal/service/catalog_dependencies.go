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
		for _, entry := range cat.Managed {
			if entry.Provider != "modrinth" ||
				entry.ProjectID == target.ProjectID ||
				strings.TrimSpace(entry.VersionID) == "" {
				continue
			}
			version, err := client.GetVersion(ctx, entry.VersionID)
			if err != nil {
				return nil, fmt.Errorf(
					"verify installed dependency metadata for %s: %w",
					entry.Name,
					err,
				)
			}
			for _, dependency := range version.Dependencies {
				if dependency.DependencyType != "required" {
					continue
				}
				projectID := strings.TrimSpace(dependency.ProjectID)
				if projectID == "" && strings.TrimSpace(dependency.VersionID) != "" {
					dependencyVersion, err := client.GetVersion(ctx, dependency.VersionID)
					if err != nil {
						return nil, fmt.Errorf(
							"resolve installed dependency of %s: %w",
							entry.Name,
							err,
						)
					}
					projectID = dependencyVersion.ProjectID
				}
				if projectID == target.ProjectID {
					appendName(entry.Name)
					break
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
			if entry.Provider != "curseforge" ||
				entry.ProjectID == target.ProjectID ||
				entry.FileID == 0 {
				continue
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
