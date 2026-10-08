package service

import (
	"sort"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func reconcileManagedSourcePaths(inv inventory.Inventory, report *catalog.Report) int {
	liveByHash := map[string][]catalog.Source{}
	for _, mod := range inv.Mods {
		hash := strings.ToLower(strings.TrimSpace(mod.SHA512))
		if hash == "" {
			continue
		}
		group := strings.TrimSpace(mod.Group)
		if mod.Location == inventory.LocationClient && group == "" {
			group = "main"
		}
		liveByHash[hash] = append(liveByHash[hash], catalog.Source{
			Location: mod.Location,
			Group:    group,
			Path:     normalizeCatalogPath(mod.Path),
		})
	}
	for hash := range liveByHash {
		sortSources(liveByHash[hash])
	}

	managedHashOwners := map[string]int{}
	for _, entry := range report.Managed {
		hash := strings.ToLower(strings.TrimSpace(entry.SHA512))
		if hash != "" {
			managedHashOwners[hash]++
		}
	}

	changed := 0
	for index := range report.Managed {
		entry := &report.Managed[index]
		hash := strings.ToLower(strings.TrimSpace(entry.SHA512))
		if hash == "" || managedHashOwners[hash] != 1 {
			continue
		}
		live := liveByHash[hash]
		if len(live) == 0 {
			continue
		}
		current := append([]catalog.Source(nil), entry.SourcePaths...)
		for sourceIndex := range current {
			current[sourceIndex].Path = normalizeCatalogPath(current[sourceIndex].Path)
			if current[sourceIndex].Location == inventory.LocationClient && strings.TrimSpace(current[sourceIndex].Group) == "" {
				current[sourceIndex].Group = autoModpackGroupFromPath(current[sourceIndex].Path)
				if current[sourceIndex].Group == "" {
					current[sourceIndex].Group = "main"
				}
			}
		}
		sortSources(current)
		if sameSources(current, live) {
			continue
		}
		entry.SourcePaths = append([]catalog.Source(nil), live...)
		changed++
	}
	if changed > 0 {
		report.RecalculateSummary()
	}
	return changed
}

func sortSources(sources []catalog.Source) {
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Location != sources[j].Location {
			return sources[i].Location < sources[j].Location
		}
		if sources[i].Group != sources[j].Group {
			return sources[i].Group < sources[j].Group
		}
		return sources[i].Path < sources[j].Path
	})
}

func sameSources(left, right []catalog.Source) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Location != right[index].Location ||
			left[index].Group != right[index].Group ||
			normalizeCatalogPath(left[index].Path) != normalizeCatalogPath(right[index].Path) {
			return false
		}
	}
	return true
}

func (s *Service) reconcileCatalogWithInventory(inv inventory.Inventory) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := reconcileManagedSourcePaths(inv, &s.state.Catalog)
	changed += adoptExactInventoryMatches(inv, &s.state.Catalog)
	if changed > 0 {
		changed += reconcileManagedSourcePaths(inv, &s.state.Catalog)
	}
	if changed == 0 {
		return 0, nil
	}
	s.state.UpdatedAt = time.Now().UTC()
	if err := s.persistState(); err != nil {
		return 0, err
	}
	return changed, nil
}

// adoptExactInventoryMatches accepts only provider-verified file identities.
// It does not guess by filename or mod metadata, and never overrides an
// explicitly unmanaged artifact or an installed different version.
func adoptExactInventoryMatches(inv inventory.Inventory, report *catalog.Report) int {
	if !inv.ModrinthChecked {
		return 0
	}
	managedHashes := make(map[string]bool)
	pinnedHashes := make(map[string]bool)
	liveHashes := make(map[string]bool)
	for _, mod := range inv.Mods {
		liveHashes[strings.ToLower(mod.SHA512)] = true
	}
	for _, entry := range report.Managed {
		managedHashes[strings.ToLower(entry.SHA512)] = true
	}
	for _, entry := range report.Pinned {
		pinnedHashes[strings.ToLower(entry.SHA512)] = true
	}
	var fresh inventory.Inventory
	fresh.SchemaVersion = inv.SchemaVersion
	fresh.ModrinthChecked = true
	for _, mod := range inv.Mods {
		hash := strings.ToLower(mod.SHA512)
		if hash == "" || managedHashes[hash] || pinnedHashes[hash] {
			continue
		}
		if mod.Modrinth == nil && mod.CurseForge == nil {
			continue
		}
		fresh.Mods = append(fresh.Mods, mod)
	}
	if len(fresh.Mods) == 0 {
		return 0
	}
	built, err := catalog.Build(fresh)
	if err != nil {
		return 0
	}
	changed := 0
	for _, entry := range built.Report.Managed {
		hash := strings.ToLower(entry.SHA512)
		if managedHashes[hash] || pinnedHashes[hash] {
			continue
		}
		owner := -1
		ambiguous := false
		for i, accepted := range report.Managed {
			if accepted.Provider != entry.Provider || accepted.ProjectID != entry.ProjectID {
				continue
			}
			if owner >= 0 {
				ambiguous = true
				break
			}
			owner = i
		}
		if ambiguous {
			continue
		}
		if owner >= 0 {
			old := report.Managed[owner]
			// A second live version of the same project is not automatically
			// adopted as that can conceal a real duplicate-version conflict.
			if liveHashes[strings.ToLower(old.SHA512)] {
				continue
			}
			entry.ArtifactID = old.ArtifactID
			entry.Deployment = old.Deployment
			entry.AutoModpackGroup = old.AutoModpackGroup
			entry.Side = old.Side
			report.Managed[owner] = entry
		} else {
			report.Managed = append(report.Managed, entry)
		}
		// Replace unresolved state for this exact byte sequence, if any.
		unresolved := report.Unresolved[:0]
		for _, unresolvedEntry := range report.Unresolved {
			if !strings.EqualFold(unresolvedEntry.SHA512, entry.SHA512) {
				unresolved = append(unresolved, unresolvedEntry)
			}
		}
		report.Unresolved = unresolved
		managedHashes[hash] = true
		changed++
	}
	if changed > 0 {
		report.RecalculateSummary()
	}
	return changed
}
