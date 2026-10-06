package service

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func normalizeCatalogAutoModpackGroups(entries []catalog.Entry) bool {
	changed := false
	for index := range entries {
		entry := &entries[index]
		if entry.Deployment != inventory.LocationClient {
			if entry.AutoModpackGroup != "" {
				entry.AutoModpackGroup = ""
				changed = true
			}
			continue
		}
		group := strings.TrimSpace(entry.AutoModpackGroup)
		if group == "" {
			for _, source := range entry.SourcePaths {
				if source.Location != inventory.LocationClient {
					continue
				}
				if source.Group != "" {
					group = strings.TrimSpace(source.Group)
					break
				}
				if inferred := autoModpackGroupFromPath(source.Path); inferred != "" {
					group = inferred
					break
				}
			}
		}
		if group == "" {
			group = "main"
		}
		if entry.AutoModpackGroup != group {
			entry.AutoModpackGroup = group
			changed = true
		}
		for sourceIndex := range entry.SourcePaths {
			source := &entry.SourcePaths[sourceIndex]
			if source.Location != inventory.LocationClient {
				if source.Group != "" {
					source.Group = ""
					changed = true
				}
				continue
			}
			sourceGroup := strings.TrimSpace(source.Group)
			if sourceGroup == "" {
				sourceGroup = autoModpackGroupFromPath(source.Path)
			}
			if sourceGroup == "" {
				sourceGroup = "main"
			}
			if source.Group != sourceGroup {
				source.Group = sourceGroup
				changed = true
			}
		}
	}
	return changed
}

func autoModpackGroupFromPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	prefix := strings.TrimSuffix(inventory.DefaultAutoModpackHostPath, "/") + "/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" {
		return ""
	}
	return parts[0]
}

func normalizeAutoModpackGroup(deployment inventory.Location, group string) string {
	if deployment != inventory.LocationClient {
		return ""
	}
	group = strings.TrimSpace(group)
	if group == "" {
		return "main"
	}
	return group
}

func inheritClientDependencyGroups(dependencies []updatecheck.Dependency, group string) {
	group = normalizeAutoModpackGroup(inventory.LocationClient, group)
	for index := range dependencies {
		dependency := &dependencies[index]
		if dependency.Deployment == inventory.LocationClient && strings.TrimSpace(dependency.AutoModpackGroup) == "" {
			dependency.AutoModpackGroup = group
		}
		nextGroup := dependency.AutoModpackGroup
		if dependency.Deployment != inventory.LocationClient {
			nextGroup = group
		}
		inheritClientDependencyGroups(dependency.Dependencies, nextGroup)
	}
}


func validateAutoModpackGroupID(group string) error {
	group = strings.TrimSpace(group)
	if group == "" {
		return fmt.Errorf("AutoModpack group is required for client placement")
	}
	if group == "." || group == ".." || strings.ContainsAny(group, "/\\") {
		return fmt.Errorf("invalid AutoModpack group %q", group)
	}
	for _, char := range group {
		if char <= ' ' || char == ':' {
			return fmt.Errorf("invalid AutoModpack group %q", group)
		}
	}
	return nil
}

func (s *Service) autoModpackModsPath(group string) string {
	group = normalizeAutoModpackGroup(inventory.LocationClient, group)
	if path := strings.TrimSpace(s.snapshot.Inventory.ClientGroupModsPaths[group]); path != "" {
		return path
	}
	if group == "main" {
		if path := strings.TrimSpace(s.options.ClientModsPath); path != "" {
			return path
		}
		return inventory.DefaultClientModsPath
	}
	return filepath.ToSlash(filepath.Join(inventory.DefaultAutoModpackHostPath, group, "mods"))
}
