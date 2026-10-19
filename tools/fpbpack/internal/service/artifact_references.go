package service

import (
	"reflect"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
)

// Migrate editable references, never silently reuse a frozen review after merging
// identities. Alias rules are combined conservatively so pins/ignores survive.
func normalizeArtifactReferences(state *State) bool {
	state.UpdateRules = cloneRules(state.UpdateRules)
	state.PendingChanges = clonePendingChanges(state.PendingChanges)
	canonical := map[string]catalog.Entry{}
	for _, entry := range state.Catalog.Managed {
		canonical[catalog.EntryKey(entry)] = entry
		for _, alias := range entry.Aliases {
			canonical[alias] = entry
		}
	}
	changed := false
	for alias, rule := range cloneRules(state.UpdateRules) {
		entry, ok := canonical[alias]
		if !ok || alias == catalog.EntryKey(entry) {
			continue
		}
		key := catalog.EntryKey(entry)
		current := state.UpdateRules[key]
		current.IgnoreMod = current.IgnoreMod || rule.IgnoreMod
		if current.PinVersion == "" {
			current.PinVersion = rule.PinVersion
		} else if rule.PinVersion != "" && rule.PinVersion != current.PinVersion {
			current.IgnoreMod = true
		}
		current.IgnoredVersions = normalizeStrings(append(current.IgnoredVersions, rule.IgnoredVersions...))
		if rule.ReviewAfter != nil && (current.ReviewAfter == nil || current.ReviewAfter.Before(*rule.ReviewAfter)) {
			current.ReviewAfter = rule.ReviewAfter
		}
		state.UpdateRules[key] = current
		delete(state.UpdateRules, alias)
		changed = true
	}
	for index := range state.PendingChanges.Changes {
		change := &state.PendingChanges.Changes[index]
		entry, ok := canonical[change.CandidateKey]
		if !ok {
			continue
		}
		key := catalog.EntryKey(entry)
		if change.CandidateKey == key {
			continue
		}
		change.CandidateKey = key
		change.ID = key
		if change.Action == "remove" && change.CatalogRequest != nil {
			change.ID += ":" + normalizeCatalogPath(change.CatalogRequest.Path)
		}
		if change.Action == "update" && strings.TrimSpace(change.Path) == "" {
			change.Placement = entry.Deployment
			change.AutoModpackGroup = entry.AutoModpackGroup
		}
		changed = true
	}
	if changed {
		// Identical updates can have been selected through both formerly separate rows.
		changes := []PendingChange{}
		for _, change := range state.PendingChanges.Changes {
			duplicate := false
			for _, accepted := range changes {
				if reflect.DeepEqual(accepted, change) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				changes = append(changes, change)
			}
		}
		state.PendingChanges.Changes = changes
		state.PendingChanges.Revision++
		state.PendingChanges.ReviewedPlanID = ""
		now := time.Now().UTC()
		state.PendingChanges.UpdatedAt = &now
	}
	return changed
}
