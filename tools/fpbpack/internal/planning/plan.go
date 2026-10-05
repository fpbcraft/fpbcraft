package planning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

const SchemaVersion = 1

var ErrNotFound = errors.New("plan not found")

type Status string

const (
	StatusReady   Status = "ready"
	StatusBlocked Status = "blocked"
)

type Finding struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	CandidateKey string `json:"candidate_key,omitempty"`
}

type Artifact struct {
	Provider   string `json:"provider"`
	ProjectID  string `json:"project_id"`
	VersionID  string `json:"version_id"`
	Filename   string `json:"filename"`
	URL        string `json:"url"`
	SHA512     string `json:"sha512"`
	Deployment string `json:"deployment"`
}

type FileOperation struct {
	Action        string `json:"action"`
	CurrentPath   string `json:"current_path,omitempty"`
	TargetPath    string `json:"target_path"`
	CurrentSHA512 string `json:"current_sha512,omitempty"`
	TargetSHA512  string `json:"target_sha512"`
}

type Change struct {
	CandidateKey     string                     `json:"candidate_key"`
	Name             string                     `json:"name"`
	Requested        bool                       `json:"requested"`
	DependencyDriven bool                       `json:"dependency_driven"`
	Classification   updatecheck.Classification `json:"classification"`
	Installed        updatecheck.Release        `json:"installed"`
	Target           updatecheck.Release        `json:"target"`
	Artifact         Artifact                   `json:"artifact"`
	Operations       []FileOperation            `json:"operations"`
}

type PrefetchedArtifact struct {
	Filename  string `json:"filename"`
	SHA512    string `json:"sha512"`
	CachePath string `json:"cache_path"`
	Bytes     int64  `json:"bytes"`
}

type Plan struct {
	SchemaVersion        int                  `json:"schema_version"`
	ID                   string               `json:"id"`
	CreatedAt            time.Time            `json:"created_at"`
	Status               Status               `json:"status"`
	InventoryGeneratedAt time.Time            `json:"inventory_generated_at"`
	UpdatesGeneratedAt   time.Time            `json:"updates_generated_at"`
	Selected             []string             `json:"selected"`
	Changes              []Change             `json:"changes"`
	Warnings             []Finding            `json:"warnings,omitempty"`
	Blockers             []Finding            `json:"blockers,omitempty"`
	Prefetched           []PrefetchedArtifact `json:"prefetched,omitempty"`
	Verified             bool                 `json:"verified"`
	VerifiedAt           *time.Time            `json:"verified_at,omitempty"`
	BackupID             string                `json:"backup_id,omitempty"`
	RequiresServerStop   bool                  `json:"requires_server_stop"`
	RequiresBackup       bool                  `json:"requires_backup"`
}

type Summary struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Status    Status    `json:"status"`
	Changes   int       `json:"changes"`
	Blockers  int       `json:"blockers"`
	Warnings  int       `json:"warnings"`
	Verified  bool      `json:"verified"`
}

type HistoryEvent struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	PlanID    string    `json:"plan_id,omitempty"`
	BackupID  string    `json:"backup_id,omitempty"`
	Mods      int       `json:"mods"`
	Summary   string    `json:"summary"`
}

func Build(selected []string, report updatecheck.Report, snapshot management.Snapshot, now time.Time) (Plan, error) {
	keys := normalizeSelection(selected)
	if len(keys) == 0 {
		return Plan{}, fmt.Errorf("at least one update candidate is required")
	}

	plan := Plan{
		SchemaVersion:        SchemaVersion,
		CreatedAt:            now.UTC(),
		Status:               StatusReady,
		InventoryGeneratedAt: snapshot.Inventory.GeneratedAt,
		UpdatesGeneratedAt:   report.GeneratedAt,
		Selected:             keys,
	}

	candidates := make(map[string]updatecheck.Candidate, len(report.Candidates))
	for _, candidate := range report.Candidates {
		candidates[candidate.Key] = candidate
	}
	mods := make(map[string]management.Mod, len(snapshot.Mods))
	for _, mod := range snapshot.Mods {
		if mod.Provider != "" && mod.ProjectID != "" {
			mods[mod.Provider+":"+mod.ProjectID] = mod
		}
	}

	changeIndex := map[string]int{}
	closureVisited := map[string]bool{}

	for _, key := range keys {
		candidate, ok := candidates[key]
		if !ok {
			addBlocker(&plan, "candidate_not_found", key, "The selected candidate is no longer present in the current update report.")
			continue
		}
		if candidate.Classification != updatecheck.ClassificationSafe &&
			candidate.Classification != updatecheck.ClassificationReview {
			addBlocker(&plan, "candidate_not_plannable", key, "Only Safe and Review candidates can be included in an update plan.")
			continue
		}
		if candidate.Target == nil {
			addBlocker(&plan, "target_missing", key, "The selected candidate does not have a resolved target release.")
			continue
		}

		target := *candidate.Target
		validateTargetArtifact(&plan, key, candidate.Name, target)

		mod, installed := mods[candidate.Provider+":"+candidate.ProjectID]
		if !installed {
			addBlocker(&plan, "installed_artifact_missing", key, "The currently managed artifact could not be matched to the live inventory.")
		}

		targetPath := target.Filename
		if installed && mod.Path != "" {
			targetPath = filepath.ToSlash(filepath.Join(filepath.Dir(mod.Path), target.Filename))
		}
		change := Change{
			CandidateKey:   key,
			Name:           candidate.Name,
			Requested:      true,
			Classification: candidate.Classification,
			Installed:      candidate.Installed,
			Target:         target,
			Artifact: Artifact{
				Provider: candidate.Provider, ProjectID: candidate.ProjectID,
				VersionID: target.ID, Filename: target.Filename, URL: target.URL,
				SHA512: target.SHA512, Deployment: string(candidate.Deployment),
			},
			Operations: []FileOperation{{
				Action: "replace", CurrentPath: mod.Path, TargetPath: targetPath,
				CurrentSHA512: mod.SHA512, TargetSHA512: target.SHA512,
			}},
		}
		appendChange(&plan, change, changeIndex)

		if candidate.Classification == updatecheck.ClassificationReview {
			message := "This update is classified for review."
			if len(candidate.Reasons) > 0 {
				message = candidate.Reasons[0].Message
			}
			plan.Warnings = append(plan.Warnings, Finding{
				Code: "review_candidate", CandidateKey: key, Message: message,
			})
		}

		appendDependencyClosure(
			&plan,
			candidate.Dependencies,
			mods,
			snapshot.Inventory,
			changeIndex,
			closureVisited,
			key,
		)
	}

	for _, finding := range snapshot.Diagnostics.Findings {
		if finding.Level != "blocking" {
			continue
		}
		message := finding.Message
		if finding.Mod != "" {
			message = finding.Mod + ": " + message
		}
		addBlocker(&plan, "diagnostic_"+finding.Code, "", message)
	}

	sort.Slice(plan.Changes, func(i, j int) bool {
		if strings.ToLower(plan.Changes[i].Name) != strings.ToLower(plan.Changes[j].Name) {
			return strings.ToLower(plan.Changes[i].Name) < strings.ToLower(plan.Changes[j].Name)
		}
		return plan.Changes[i].CandidateKey < plan.Changes[j].CandidateKey
	})
	plan.Warnings = uniqueFindings(plan.Warnings)
	plan.Blockers = uniqueFindings(plan.Blockers)
	plan.RequiresServerStop = len(plan.Changes) > 0
	plan.RequiresBackup = requiresBackup(plan.Changes)
	if len(plan.Blockers) > 0 {
		plan.Status = StatusBlocked
	}
	plan.ID = planID(plan)
	return plan, nil
}

func appendDependencyClosure(
	plan *Plan,
	dependencies []updatecheck.Dependency,
	mods map[string]management.Mod,
	inv inventory.Inventory,
	changeIndex map[string]int,
	visited map[string]bool,
	parentKey string,
) {
	for _, dependency := range dependencies {
		depKey := dependency.Provider + ":" + dependency.ProjectID
		switch dependency.Action {
		case "unresolved", "conflict":
			addBlocker(plan, "dependency_"+dependency.Action, parentKey, "A required dependency cannot be resolved safely for this target.")
			continue
		case "add", "update":
			if dependency.Target == nil {
				addBlocker(plan, "dependency_target_not_resolved", parentKey, "A required dependency change does not have an exact target artifact.")
				continue
			}
			target := *dependency.Target
			validateTargetArtifact(plan, depKey, dependency.Name, target)

			visitKey := depKey + ":" + target.ID
			if visited[visitKey] {
				continue
			}
			visited[visitKey] = true

			name := dependency.Name
			if name == "" {
				name = dependency.ProjectID
			}
			deployment := dependency.Deployment
			if deployment == "" {
				deployment = inventory.LocationServer
			}

			mod, installed := mods[depKey]
			operation := FileOperation{
				Action: "add",
				TargetPath: filepath.ToSlash(filepath.Join(modsPath(inv, deployment), target.Filename)),
				TargetSHA512: target.SHA512,
			}
			installedRelease := updatecheck.Release{}
			if dependency.Action == "update" {
				if !installed {
					addBlocker(plan, "dependency_installed_artifact_missing", depKey, "A dependency update was resolved, but the installed dependency could not be matched to the live inventory.")
				} else {
					operation.Action = "replace"
					operation.CurrentPath = mod.Path
					operation.TargetPath = filepath.ToSlash(filepath.Join(filepath.Dir(mod.Path), target.Filename))
					operation.CurrentSHA512 = mod.SHA512
					installedRelease = updatecheck.Release{
						ID: dependency.InstalledVersion,
						Name: mod.Name,
						Filename: mod.Filename,
						SHA512: mod.SHA512,
					}
				}
			} else if installed {
				addBlocker(plan, "dependency_state_changed", depKey, "A dependency was resolved as an addition but is now present in the live inventory.")
			}

			change := Change{
				CandidateKey: depKey,
				Name: name,
				DependencyDriven: true,
				Classification: updatecheck.ClassificationReview,
				Installed: installedRelease,
				Target: target,
				Artifact: Artifact{
					Provider: dependency.Provider,
					ProjectID: dependency.ProjectID,
					VersionID: target.ID,
					Filename: target.Filename,
					URL: target.URL,
					SHA512: target.SHA512,
					Deployment: string(deployment),
				},
				Operations: []FileOperation{operation},
			}
			appendChange(plan, change, changeIndex)
			appendDependencyClosure(
				plan,
				dependency.Dependencies,
				mods,
				inv,
				changeIndex,
				visited,
				depKey,
			)
		}
	}
}

func appendChange(plan *Plan, change Change, changeIndex map[string]int) {
	key := change.Artifact.Provider + ":" + change.Artifact.ProjectID
	if index, exists := changeIndex[key]; exists {
		current := &plan.Changes[index]
		if current.Target.ID != change.Target.ID {
			addBlocker(
				plan,
				"dependency_target_conflict",
				key,
				fmt.Sprintf("The selected updates require conflicting target versions for %s.", change.Name),
			)
			return
		}
		if len(current.Operations) != len(change.Operations) ||
			(len(current.Operations) > 0 && len(change.Operations) > 0 &&
				(current.Operations[0].Action != change.Operations[0].Action ||
					current.Operations[0].TargetPath != change.Operations[0].TargetPath)) {
			addBlocker(plan, "dependency_operation_conflict", key, "The same artifact resolved to conflicting filesystem operations.")
			return
		}
		current.Requested = current.Requested || change.Requested
		current.DependencyDriven = current.DependencyDriven || change.DependencyDriven
		if change.Requested {
			current.Name = change.Name
			current.Classification = change.Classification
			current.Installed = change.Installed
		}
		return
	}
	changeIndex[key] = len(plan.Changes)
	plan.Changes = append(plan.Changes, change)
}

func validateTargetArtifact(plan *Plan, key, name string, target updatecheck.Release) {
	if target.URL == "" || target.SHA512 == "" || target.Filename == "" {
		if name == "" {
			name = key
		}
		addBlocker(
			plan,
			"target_artifact_incomplete",
			key,
			"The target release for "+name+" is missing a download URL, SHA-512, or filename.",
		)
	}
}

func addBlocker(plan *Plan, code, candidateKey, message string) {
	plan.Blockers = append(plan.Blockers, Finding{
		Code: code, CandidateKey: candidateKey, Message: message,
	})
}

func modsPath(inv inventory.Inventory, deployment inventory.Location) string {
	if deployment == inventory.LocationClient {
		if strings.TrimSpace(inv.ClientModsPath) != "" {
			return inv.ClientModsPath
		}
		return inventory.DefaultClientModsPath
	}
	if strings.TrimSpace(inv.ServerModsPath) != "" {
		return inv.ServerModsPath
	}
	return inventory.DefaultServerModsPath
}

func requiresBackup(changes []Change) bool {
	for _, change := range changes {
		for _, operation := range change.Operations {
			if operation.CurrentPath != "" && (operation.Action == "replace" || operation.Action == "remove") {
				return true
			}
		}
	}
	return false
}

func (p Plan) Summary() Summary {
	return Summary{
		ID: p.ID, CreatedAt: p.CreatedAt, Status: p.Status, Changes: len(p.Changes),
		Blockers: len(p.Blockers), Warnings: len(p.Warnings), Verified: p.Verified,
	}
}

func (p Plan) HistoryEvent() HistoryEvent {
	return HistoryEvent{
		ID: "plan:" + p.ID,
		CreatedAt: p.CreatedAt,
		Type: "plan",
		Status: string(p.Status),
		PlanID: p.ID,
		BackupID: p.BackupID,
		Mods: len(p.Changes),
		Summary: fmt.Sprintf("Update plan with %d mod change(s)", len(p.Changes)),
	}
}

func normalizeSelection(selected []string) []string {
	seen := map[string]struct{}{}
	keys := make([]string, 0, len(selected))
	for _, key := range selected {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func uniqueFindings(findings []Finding) []Finding {
	seen := map[string]struct{}{}
	result := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		key := finding.Code + "\x00" + finding.CandidateKey + "\x00" + finding.Message
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, finding)
	}
	return result
}

func planID(plan Plan) string {
	payload := struct {
		InventoryGeneratedAt time.Time `json:"inventory_generated_at"`
		UpdatesGeneratedAt   time.Time `json:"updates_generated_at"`
		Selected             []string  `json:"selected"`
		Changes              []Change  `json:"changes"`
		Blockers             []Finding `json:"blockers"`
	}{
		InventoryGeneratedAt: plan.InventoryGeneratedAt,
		UpdatesGeneratedAt: plan.UpdatesGeneratedAt,
		Selected: plan.Selected,
		Changes: plan.Changes,
		Blockers: plan.Blockers,
	}
	bytes, _ := json.Marshal(payload)
	sum := sha256.Sum256(bytes)
	return "plan-" + hex.EncodeToString(sum[:8])
}
