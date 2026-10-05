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
	RequiresServerStop   bool                 `json:"requires_server_stop"`
	RequiresBackup       bool                 `json:"requires_backup"`
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

	for _, key := range keys {
		candidate, ok := candidates[key]
		if !ok {
			plan.Blockers = append(plan.Blockers, Finding{
				Code: "candidate_not_found", CandidateKey: key,
				Message: "The selected candidate is no longer present in the current update report.",
			})
			continue
		}
		if candidate.Classification != updatecheck.ClassificationSafe &&
			candidate.Classification != updatecheck.ClassificationReview {
			plan.Blockers = append(plan.Blockers, Finding{
				Code: "candidate_not_plannable", CandidateKey: key,
				Message: "Only Safe and Review candidates can be included in an update plan.",
			})
			continue
		}
		if candidate.Target == nil {
			plan.Blockers = append(plan.Blockers, Finding{
				Code: "target_missing", CandidateKey: key,
				Message: "The selected candidate does not have a resolved target release.",
			})
			continue
		}

		target := *candidate.Target
		if target.URL == "" || target.SHA512 == "" || target.Filename == "" {
			plan.Blockers = append(plan.Blockers, Finding{
				Code: "target_artifact_incomplete", CandidateKey: key,
				Message: "The target release is missing a download URL, SHA-512, or filename.",
			})
		}

		mod, installed := mods[candidate.Provider+":"+candidate.ProjectID]
		if !installed {
			plan.Blockers = append(plan.Blockers, Finding{
				Code: "installed_artifact_missing", CandidateKey: key,
				Message: "The currently managed artifact could not be matched to the live inventory.",
			})
		}

		for _, dependency := range candidate.Dependencies {
			switch dependency.Action {
			case "unresolved", "conflict":
				plan.Blockers = append(plan.Blockers, Finding{
					Code: "dependency_" + dependency.Action, CandidateKey: key,
					Message: "A required dependency cannot be resolved safely for this target.",
				})
			case "add", "update":
				plan.Blockers = append(plan.Blockers, Finding{
					Code: "dependency_artifact_not_resolved", CandidateKey: key,
					Message: "A dependency change is required, but its exact target artifact is not yet part of the candidate closure.",
				})
			}
		}

		if candidate.Classification == updatecheck.ClassificationReview {
			message := "This update is classified for review."
			if len(candidate.Reasons) > 0 {
				message = candidate.Reasons[0].Message
			}
			plan.Warnings = append(plan.Warnings, Finding{
				Code: "review_candidate", CandidateKey: key, Message: message,
			})
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
		plan.Changes = append(plan.Changes, change)
	}

	for _, finding := range snapshot.Diagnostics.Findings {
		if finding.Level != "blocking" {
			continue
		}
		message := finding.Message
		if finding.Mod != "" {
			message = finding.Mod + ": " + message
		}
		plan.Blockers = append(plan.Blockers, Finding{
			Code: "diagnostic_" + finding.Code,
			Message: message,
		})
	}

	sort.Slice(plan.Changes, func(i, j int) bool {
		return strings.ToLower(plan.Changes[i].Name) < strings.ToLower(plan.Changes[j].Name)
	})
	plan.Warnings = uniqueFindings(plan.Warnings)
	plan.Blockers = uniqueFindings(plan.Blockers)
	plan.RequiresServerStop = len(plan.Changes) > 0
	plan.RequiresBackup = len(plan.Changes) > 0
	if len(plan.Blockers) > 0 {
		plan.Status = StatusBlocked
	}
	plan.ID = planID(plan)
	return plan, nil
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
