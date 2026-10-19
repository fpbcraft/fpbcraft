package planning

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func indexManagedMods(files []management.Mod) map[string]management.Mod {
	ordered := append([]management.Mod(nil), files...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	result := map[string]management.Mod{}
	projects := map[string][]management.Mod{}
	for _, mod := range ordered {
		if mod.Provider == "" || mod.ProjectID == "" {
			continue
		}
		base := mod.Provider + ":" + mod.ProjectID
		projects[base] = append(projects[base], mod)
		if mod.ID != "" {
			if _, ok := result[mod.ID]; !ok {
				result[mod.ID] = mod
			}
		}
		for _, alias := range mod.Aliases {
			result[alias] = mod
		}
	}
	for key, copies := range projects {
		unique := true
		for _, mod := range copies[1:] {
			if copies[0].SHA512 == "" || !strings.EqualFold(copies[0].SHA512, mod.SHA512) {
				unique = false
				break
			}
		}
		if unique {
			result[key] = copies[0]
		}
	}
	return result
}

// Updates act on releases; placement/removal actions can address one physical file.
func expandArtifactPlacements(plan *Plan, candidates map[string]updatecheck.Candidate, mods []management.Mod) {
	for index := range plan.Changes {
		change := &plan.Changes[index]
		candidate := candidates[change.CandidateKey]
		if ((candidate.SourcePath != "" || len(candidate.SourcePaths) > 0) && (candidate.Intent == "placement" || candidate.Intent == "remove")) || len(change.Operations) != 1 {
			continue
		}
		original := change.Operations[0]
		if original.Action != "replace" && original.Action != "remove" {
			continue
		}
		copies := []management.Mod{}
		for _, mod := range mods {
			sameIdentity := mod.ID == change.CandidateKey || mod.Provider+":"+mod.ProjectID == change.CandidateKey
			for _, alias := range mod.Aliases {
				sameIdentity = sameIdentity || alias == change.CandidateKey
			}
			if sameIdentity && strings.EqualFold(mod.SHA512, original.CurrentSHA512) {
				copies = append(copies, mod)
			}
		}
		if len(copies) < 2 {
			continue
		}
		sort.Slice(copies, func(i, j int) bool { return copies[i].Path < copies[j].Path })
		change.Operations = nil
		for _, mod := range copies {
			operation := original
			operation.CurrentPath = mod.Path
			operation.CurrentSHA512 = mod.SHA512
			if operation.Action == "remove" {
				operation.TargetPath = mod.Path
			} else {
				if candidate.SourcePath == mod.Path {
					operation.TargetPath = original.TargetPath
				} else {
					operation.TargetPath = filepath.ToSlash(filepath.Join(filepath.Dir(mod.Path), change.Artifact.Filename))
				}
			}
			change.Operations = append(change.Operations, operation)
		}
		coalesceReleaseTargets(change)
	}
}

func coalescePlacementTargets(plan *Plan, mods []management.Mod) {
	byPath := map[string]management.Mod{}
	for _, mod := range mods {
		byPath[mod.Path] = mod
	}
	for ci := range plan.Changes {
		change := &plan.Changes[ci]
		for oi := range change.Operations {
			op := &change.Operations[oi]
			if op.Action != "replace" || op.CurrentPath == op.TargetPath || op.CurrentSHA512 == "" || !strings.EqualFold(op.CurrentSHA512, op.TargetSHA512) {
				continue
			}
			target, ok := byPath[op.TargetPath]
			if !ok || target.Management != "managed" || !strings.EqualFold(target.SHA512, op.TargetSHA512) || target.Provider != change.Artifact.Provider || target.ProjectID != change.Artifact.ProjectID {
				continue
			}
			// The destination is already correct. Back up and remove just the redundant source.
			op.Action = "remove"
			op.TargetPath = op.CurrentPath
			op.TargetSHA512 = ""
		}
	}
}

// A move plus version update can collapse two old locations onto one new path.
// Keep the destination's writer and remove only the redundant old source.
func coalesceReleaseTargets(change *Change) {
	owners := map[string]int{}
	for index, op := range change.Operations {
		if op.Action == "remove" {
			continue
		}
		previous, exists := owners[op.TargetPath]
		if !exists {
			owners[op.TargetPath] = index
			continue
		}
		removed := index
		if filepath.Dir(op.CurrentPath) == filepath.Dir(op.TargetPath) {
			removed = previous
			owners[op.TargetPath] = index
		}
		old := &change.Operations[removed]
		old.Action = "remove"
		old.TargetPath = old.CurrentPath
		old.TargetSHA512 = ""
	}
}

func validateArtifactSources(plan *Plan, snapshot management.Snapshot) {
	files := map[string]management.Mod{}
	for _, file := range snapshot.Mods {
		files[file.Path] = file
	}
	for _, change := range plan.Changes {
		writes := false
		for _, op := range change.Operations {
			if op.Action != "remove" {
				writes = true
			}
		}
		if !writes {
			continue
		}
		for _, entry := range snapshot.Catalog.Managed {
			if !catalog.MatchesKey(entry, change.CandidateKey) {
				continue
			}
			for _, source := range entry.SourcePaths {
				file, ok := files[source.Path]
				if !ok || !strings.EqualFold(file.SHA512, entry.SHA512) {
					plan.Blockers = append(plan.Blockers, Finding{Code: "artifact_placement_drift", CandidateKey: change.CandidateKey, Name: change.Name, Path: source.Path, Message: "An intended artifact location is missing or has different bytes; reconcile it before updating this release."})
				}
			}
		}
	}
}
