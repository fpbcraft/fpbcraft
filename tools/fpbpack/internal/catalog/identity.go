package catalog

import (
	"path/filepath"
	"sort"
	"strings"
)

// CoalesceManaged merges only exact, provider-verified artifact identities.
// Physical placements and retired keys survive so pending references can be resolved.
func (r *Report) CoalesceManaged() bool {
	entries := make([]Entry, 0, len(r.Managed))
	owners := map[string]int{}
	changed := false
	for _, entry := range r.Managed {
		hash := strings.ToLower(strings.TrimSpace(entry.SHA512))
		key := entry.Provider + ":" + entry.ProjectID + ":" + hash
		if hash == "" {
			entries = append(entries, entry)
			continue
		}
		index, exists := owners[key]
		if !exists {
			owners[key] = len(entries)
			entries = append(entries, entry)
			continue
		}
		owner := &entries[index]
		for _, source := range entry.SourcePaths {
			found := false
			for _, previous := range owner.SourcePaths {
				if filepath.ToSlash(filepath.Clean(previous.Path)) == filepath.ToSlash(filepath.Clean(source.Path)) {
					found = true
					break
				}
			}
			if !found {
				owner.SourcePaths = append(owner.SourcePaths, source)
			}
		}
		for _, alias := range append([]string{EntryKey(entry)}, entry.Aliases...) {
			if alias == EntryKey(*owner) {
				continue
			}
			found := false
			for _, old := range owner.Aliases {
				if old == alias {
					found = true
					break
				}
			}
			if !found {
				owner.Aliases = append(owner.Aliases, alias)
			}
		}
		sort.Strings(owner.Aliases)
		sort.Slice(owner.SourcePaths, func(i, j int) bool { return owner.SourcePaths[i].Path < owner.SourcePaths[j].Path })
		changed = true
	}
	if changed {
		r.Managed = entries
	}
	return changed
}

func MatchesKey(entry Entry, key string) bool {
	if EntryKey(entry) == key {
		return true
	}
	for _, alias := range entry.Aliases {
		if alias == key {
			return true
		}
	}
	return false
}

// UniqueProjects marks ambiguous projects without choosing a version by order.
func UniqueProjects(entries []Entry, provider string) map[string]Entry {
	result := map[string]Entry{}
	ambiguous := map[string]bool{}
	for _, entry := range entries {
		if entry.Provider != provider {
			continue
		}
		if previous, ok := result[entry.ProjectID]; ok &&
			(previous.SHA512 == "" || !strings.EqualFold(previous.SHA512, entry.SHA512)) {
			ambiguous[entry.ProjectID] = true
		}
		result[entry.ProjectID] = entry
	}
	for project := range ambiguous {
		result[project] = Entry{Provider: provider, ProjectID: project, Name: "ambiguous installed versions"}
	}
	return result
}
