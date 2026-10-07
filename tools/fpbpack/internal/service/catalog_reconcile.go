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
	if changed == 0 {
		return 0, nil
	}
	s.state.UpdatedAt = time.Now().UTC()
	if err := s.persistState(); err != nil {
		return 0, err
	}
	return changed, nil
}
