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
	Name         string `json:"name,omitempty"`
	Path         string `json:"path,omitempty"`
}

type Artifact struct {
	Provider   string `json:"provider"`
	ProjectID  string `json:"project_id"`
	VersionID  string `json:"version_id"`
	Filename   string `json:"filename"`
	URL        string `json:"url"`
	SHA1       string `json:"sha1,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	SHA512     string `json:"sha512,omitempty"`
	Deployment       string `json:"deployment"`
	AutoModpackGroup string `json:"automodpack_group,omitempty"`
	Environment    string `json:"environment,omitempty"`
	ManualDownload bool   `json:"manual_download,omitempty"`
	ManualProvided bool   `json:"manual_provided,omitempty"`
	ManualURL      string `json:"manual_url,omitempty"`
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
	PendingRevision      uint64               `json:"pending_revision,omitempty"`
	Changes              []Change             `json:"changes"`
	Warnings             []Finding            `json:"warnings,omitempty"`
	Blockers             []Finding            `json:"blockers,omitempty"`
	Prefetched           []PrefetchedArtifact `json:"prefetched,omitempty"`
	Verified             bool                 `json:"verified"`
	VerifiedAt           *time.Time            `json:"verified_at,omitempty"`
	BackupID             string                `json:"backup_id,omitempty"`
	AppliedAt            *time.Time             `json:"applied_at,omitempty"`
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
	mods := make(map[string]management.Mod, len(snapshot.Mods)*2)
	projectCounts := map[string]int{}
	for _, mod := range snapshot.Mods {
		if mod.Provider != "" && mod.ProjectID != "" {
			projectCounts[mod.Provider+":"+mod.ProjectID]++
		}
	}
	for _, mod := range snapshot.Mods {
		if mod.Provider == "" || mod.ProjectID == "" {
			continue
		}
		base := mod.Provider + ":" + mod.ProjectID
		if strings.TrimSpace(mod.ID) != "" {
			mods[mod.ID] = mod
		}
		if projectCounts[base] == 1 || strings.TrimSpace(mod.ID) == "" {
			mods[base] = mod
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
			addBlocker(&plan, "candidate_not_plannable", key, "Only Safe and Review candidates can be included in pending changes.")
			continue
		}
		intent := strings.TrimSpace(candidate.Intent)
		if intent == "" {
			intent = "update"
		}
		mod, installed := mods[candidate.Key]

		if intent == "remove" {
			if !installed {
				addBlocker(&plan, "installed_artifact_missing", key, "The managed artifact selected for removal could not be matched to the live inventory.")
				continue
			}
			if len(candidate.RequiredBy) > 0 {
				addBlocker(
					&plan,
					"required_dependency_remove",
					key,
					fmt.Sprintf(
						"%s is still required by %s. Remove or change those dependents first.",
						candidate.Name,
						strings.Join(candidate.RequiredBy, ", "),
					),
				)
			}
			change := Change{
				CandidateKey:   key,
				Name:           candidate.Name,
				Requested:      true,
				Classification: candidate.Classification,
				Installed:      candidate.Installed,
				Target:         candidate.Installed,
				Artifact: Artifact{
					Provider: candidate.Provider,
					ProjectID: candidate.ProjectID,
					VersionID: candidate.Installed.ID,
					Filename: mod.Filename,
					SHA512: mod.SHA512,
					Deployment: string(mod.Deployment),
					AutoModpackGroup: normalizedGroup(mod.Deployment, mod.AutoModpackGroup),
				},
				Operations: []FileOperation{{
					Action: "remove",
					CurrentPath: mod.Path,
					TargetPath: mod.Path,
					CurrentSHA512: mod.SHA512,
				}},
			}
			appendChange(&plan, change, changeIndex)
			plan.Warnings = append(plan.Warnings, Finding{
				Code: "catalog_remove",
				CandidateKey: key,
				Message: "These changes remove the managed JAR and its accepted catalog entry. Restore can put both back.",
			})
			continue
		}

		if candidate.Target == nil {
			addBlocker(&plan, "target_missing", key, "The selected candidate does not have a resolved target release.")
			continue
		}

		target := *candidate.Target
		if intent == "placement" {
			if target.Filename == "" || target.SHA512 == "" {
				addBlocker(
					&plan,
					"placement_artifact_incomplete",
					key,
					"A placement move requires the current managed filename and SHA-512 so the installed bytes can be verified and reused.",
				)
			}
		} else {
			validateTargetArtifact(&plan, key, candidate.Name, target)
		}

		action := "replace"
		currentPath := mod.Path
		currentSHA512 := mod.SHA512
		if intent == "install" {
			action = "add"
			currentPath = ""
			currentSHA512 = ""
			if installed {
				addBlocker(&plan, "artifact_already_installed", key, "This provider project is already managed in the live inventory.")
			}
		} else if !installed {
			addBlocker(&plan, "installed_artifact_missing", key, "The currently managed artifact could not be matched to the live inventory.")
		}

		targetPath := filepath.ToSlash(filepath.Join(modsPath(snapshot.Inventory, candidate.Deployment, candidate.AutoModpackGroup), target.Filename))
		if installed && mod.Path != "" && mod.Deployment == candidate.Deployment &&
			(mod.Deployment != inventory.LocationClient || normalizedGroup(mod.Deployment, mod.AutoModpackGroup) == normalizedGroup(candidate.Deployment, candidate.AutoModpackGroup)) {
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
				SHA1: target.SHA1, SHA256: target.SHA256, SHA512: target.SHA512, Deployment: string(candidate.Deployment), AutoModpackGroup: normalizedGroup(candidate.Deployment, candidate.AutoModpackGroup), Environment: candidate.Environment,
				ManualDownload: target.ManualDownload, ManualURL: target.ManualURL,
			},
			Operations: []FileOperation{{
				Action: action, CurrentPath: currentPath, TargetPath: targetPath,
				CurrentSHA512: currentSHA512, TargetSHA512: target.SHA512,
			}},
		}
		appendChange(&plan, change, changeIndex)

		if candidate.Classification == updatecheck.ClassificationReview {
			message := "This catalog change is classified for review."
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
		plan.Blockers = append(plan.Blockers, Finding{
			Code:    "diagnostic_" + finding.Code,
			Message: message,
			Name:    finding.Mod,
			Path:    finding.Path,
		})
	}

	sort.Slice(plan.Changes, func(i, j int) bool {
		if strings.ToLower(plan.Changes[i].Name) != strings.ToLower(plan.Changes[j].Name) {
			return strings.ToLower(plan.Changes[i].Name) < strings.ToLower(plan.Changes[j].Name)
		}
		return plan.Changes[i].CandidateKey < plan.Changes[j].CandidateKey
	})
	coalesceSatisfiedManagedAdds(&plan, snapshot.Mods)
	validateOperationCollisions(&plan, snapshot.Mods)
	annotateFindingContexts(&plan, snapshot)
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
		name := dependency.Name
		if name == "" {
			name = dependency.ProjectID
		}
		switch dependency.Action {
		case "unresolved", "conflict":
			finding := Finding{
				Code:         "dependency_" + dependency.Action,
				CandidateKey: depKey,
				Name:         name,
				Message: fmt.Sprintf(
					"Required dependency %s cannot be resolved safely for this target.",
					name,
				),
			}
			if mod, ok := mods[depKey]; ok {
				finding.Path = mod.Path
			}
			plan.Blockers = append(plan.Blockers, finding)
			continue
		case "add", "update":
			if dependency.Target == nil {
				finding := Finding{
					Code:         "dependency_target_not_resolved",
					CandidateKey: depKey,
					Name:         name,
					Message: fmt.Sprintf(
						"Required dependency %s does not have an exact target artifact.",
						name,
					),
				}
				if mod, ok := mods[depKey]; ok {
					finding.Path = mod.Path
				}
				plan.Blockers = append(plan.Blockers, finding)
				continue
			}
			target := *dependency.Target
			validateTargetArtifact(plan, depKey, dependency.Name, target)

			visitKey := depKey + ":" + target.ID
			if visited[visitKey] {
				continue
			}
			visited[visitKey] = true

			deployment := dependency.Deployment
			if deployment == "" {
				deployment = inventory.LocationServer
			}

			mod, installed := mods[depKey]
			aliasProvider := false
			if !installed {
				mod, installed, aliasProvider = findInstalledDependencyAlias(dependency, target, mods, inv)
			}
			if installed && dependency.Action == "add" &&
				dependencyRequirementSatisfied(dependency, target, mod, inv) {
				path := mod.Path
				if strings.TrimSpace(path) == "" {
					path = mod.Filename
				}
				plan.Warnings = append(plan.Warnings, Finding{
					Code:         "dependency_already_satisfied",
					CandidateKey: depKey,
					Name:         name,
					Path:         path,
					Message: fmt.Sprintf(
						"%s is already satisfied by the managed artifact at %s; no filesystem change is needed.",
						name,
						path,
					),
				})
				continue
			}
			if installed && dependency.Action == "add" && aliasProvider {
				plan.Blockers = append(plan.Blockers, Finding{
					Code:         "dependency_cross_provider_version_mismatch",
					CandidateKey: depKey,
					Name:         name,
					Path:         mod.Path,
					Message: fmt.Sprintf(
						"%s is installed through %s, but those bytes do not satisfy the exact %s dependency target. Choose the dependency version/provider explicitly instead of installing a duplicate.",
						name,
						mod.Provider,
						dependency.Provider,
					),
				})
				continue
			}

			effectiveGroup := dependency.AutoModpackGroup
			if installed && dependencyCanRemainAt(dependency.Environment, mod.Deployment) {
				deployment = mod.Deployment
				effectiveGroup = mod.AutoModpackGroup
			}
			operation := FileOperation{
				Action: "add",
				TargetPath: filepath.ToSlash(filepath.Join(modsPath(inv, deployment, effectiveGroup), target.Filename)),
				TargetSHA512: target.SHA512,
			}
			installedRelease := updatecheck.Release{}
			if dependency.Action == "update" || installed {
				if !installed {
					addBlocker(plan, "dependency_installed_artifact_missing", depKey, "A dependency update was resolved, but the installed dependency could not be matched to the live inventory.")
				} else {
					operation.Action = "replace"
					operation.CurrentPath = mod.Path
					if mod.Deployment == deployment &&
						(deployment != inventory.LocationClient || normalizedGroup(mod.Deployment, mod.AutoModpackGroup) == normalizedGroup(deployment, effectiveGroup)) {
						operation.TargetPath = filepath.ToSlash(filepath.Join(filepath.Dir(mod.Path), target.Filename))
					}
					operation.CurrentSHA512 = mod.SHA512
					installedRelease = updatecheck.Release{
						ID:       dependency.InstalledVersion,
						Name:     mod.Name,
						Filename: mod.Filename,
						SHA1:     mod.SHA1,
						SHA512:   mod.SHA512,
					}
					if dependency.Action == "add" {
						plan.Warnings = append(plan.Warnings, Finding{
							Code:         "dependency_state_reconciled",
							CandidateKey: depKey,
							Name:         name,
							Path:         mod.Path,
							Message: fmt.Sprintf(
								"%s appeared in the live inventory after dependency resolution. The pending review was reconciled to update that artifact instead of blocking.",
								name,
							),
						})
					}
				}
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
					SHA1: target.SHA1,
					SHA256: target.SHA256,
					SHA512: target.SHA512,
					Deployment: string(deployment),
					AutoModpackGroup: normalizedGroup(deployment, effectiveGroup),
					Environment: dependency.Environment,
					ManualDownload: target.ManualDownload,
					ManualURL: target.ManualURL,
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
	key := change.CandidateKey
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
	if (!target.ManualDownload && target.URL == "") ||
		(target.SHA512 == "" && target.SHA256 == "" && target.SHA1 == "") ||
		target.Filename == "" {
		if name == "" {
			name = key
		}
		addBlocker(
			plan,
			"target_artifact_incomplete",
			key,
			"The target release for "+name+" is missing a usable download/manual source, provider checksum, or filename.",
		)
	}
}

func addBlocker(plan *Plan, code, candidateKey, message string) {
	plan.Blockers = append(plan.Blockers, Finding{
		Code: code, CandidateKey: candidateKey, Message: message,
	})
}

func modsPath(inv inventory.Inventory, deployment inventory.Location, group string) string {
	if deployment == inventory.LocationClient {
		group = normalizedGroup(deployment, group)
		if inv.ClientGroupModsPaths != nil {
			if path := strings.TrimSpace(inv.ClientGroupModsPaths[group]); path != "" {
				return path
			}
		}
		if group == "main" && strings.TrimSpace(inv.ClientModsPath) != "" {
			return inv.ClientModsPath
		}
		return filepath.ToSlash(filepath.Join(inventory.DefaultAutoModpackHostPath, group, "mods"))
	}
	if strings.TrimSpace(inv.ServerModsPath) != "" {
		return inv.ServerModsPath
	}
	return inventory.DefaultServerModsPath
}

func normalizedGroup(deployment inventory.Location, group string) string {
	if deployment != inventory.LocationClient {
		return ""
	}
	group = strings.TrimSpace(group)
	if group == "" {
		return "main"
	}
	return group
}

func findInstalledDependencyAlias(
	dependency updatecheck.Dependency,
	target updatecheck.Release,
	mods map[string]management.Mod,
	inv inventory.Inventory,
) (management.Mod, bool, bool) {
	byPath := make(map[string]management.Mod)
	for _, mod := range mods {
		if strings.TrimSpace(mod.Path) == "" || mod.Management != "managed" {
			continue
		}
		byPath[filepath.ToSlash(filepath.Clean(mod.Path))] = mod
	}

	var exact *management.Mod
	for _, mod := range byPath {
		if !releaseMatchesManagedMod(target, mod) {
			continue
		}
		if exact != nil && exact.Path != mod.Path {
			exact = nil
			break
		}
		copy := mod
		exact = &copy
	}
	if exact != nil {
		return *exact, true, exact.Provider != dependency.Provider
	}

	var matched *management.Mod
	for _, file := range inv.Mods {
		matches := false
		switch dependency.Provider {
		case "modrinth":
			matches = file.Modrinth != nil && file.Modrinth.ProjectID == dependency.ProjectID
		case "curseforge":
			matches = file.CurseForge != nil &&
				fmt.Sprintf("%d", file.CurseForge.ProjectID) == dependency.ProjectID
		}
		if !matches {
			continue
		}
		mod, ok := byPath[filepath.ToSlash(filepath.Clean(file.Path))]
		if !ok {
			continue
		}
		if matched != nil && matched.Path != mod.Path {
			return management.Mod{}, false, false
		}
		copy := mod
		matched = &copy
	}
	if matched == nil {
		return management.Mod{}, false, false
	}
	return *matched, true, matched.Provider != dependency.Provider
}

func dependencyRequirementSatisfied(
	dependency updatecheck.Dependency,
	target updatecheck.Release,
	mod management.Mod,
	inv inventory.Inventory,
) bool {
	if releaseMatchesManagedMod(target, mod) {
		return true
	}
	if strings.TrimSpace(dependency.VersionID) == "" {
		return true
	}
	for _, file := range inv.Mods {
		if filepath.ToSlash(filepath.Clean(file.Path)) != filepath.ToSlash(filepath.Clean(mod.Path)) {
			continue
		}
		switch dependency.Provider {
		case "modrinth":
			return file.Modrinth != nil &&
				file.Modrinth.ProjectID == dependency.ProjectID &&
				file.Modrinth.VersionID == dependency.VersionID
		case "curseforge":
			return file.CurseForge != nil &&
				fmt.Sprintf("%d", file.CurseForge.ProjectID) == dependency.ProjectID &&
				fmt.Sprintf("%d", file.CurseForge.FileID) == dependency.VersionID
		}
	}
	return false
}

func releaseMatchesManagedMod(target updatecheck.Release, mod management.Mod) bool {
	if strings.TrimSpace(target.SHA512) != "" &&
		strings.TrimSpace(mod.SHA512) != "" &&
		strings.EqualFold(strings.TrimSpace(target.SHA512), strings.TrimSpace(mod.SHA512)) {
		return true
	}
	return strings.TrimSpace(target.SHA1) != "" &&
		strings.TrimSpace(mod.SHA1) != "" &&
		strings.EqualFold(strings.TrimSpace(target.SHA1), strings.TrimSpace(mod.SHA1))
}

func dependencyCanRemainAt(environment string, deployment inventory.Location) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "client_only", "singleplayer_only":
		return deployment == inventory.LocationClient
	case "server_only", "dedicated_server_only":
		return deployment == inventory.LocationServer
	default:
		return true
	}
}

func coalesceSatisfiedManagedAdds(plan *Plan, mods []management.Mod) {
	liveByPath := make(map[string]management.Mod, len(mods))
	liveBySHA512 := map[string]management.Mod{}
	liveBySHA1 := map[string]management.Mod{}
	for _, mod := range mods {
		if strings.TrimSpace(mod.Path) != "" {
			liveByPath[filepath.ToSlash(filepath.Clean(mod.Path))] = mod
		}
		if mod.Management != "managed" {
			continue
		}
		if hash := strings.ToLower(strings.TrimSpace(mod.SHA512)); hash != "" {
			liveBySHA512[hash] = mod
		}
		if hash := strings.ToLower(strings.TrimSpace(mod.SHA1)); hash != "" {
			liveBySHA1[hash] = mod
		}
	}

	changes := make([]Change, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		operations := make([]FileOperation, 0, len(change.Operations))
		for _, operation := range change.Operations {
			if change.Requested || !change.DependencyDriven || operation.Action != "add" {
				operations = append(operations, operation)
				continue
			}

			var (
				live  management.Mod
				found bool
			)
			if hash := strings.ToLower(strings.TrimSpace(change.Artifact.SHA512)); hash != "" {
				live, found = liveBySHA512[hash]
			}
			if !found {
				if hash := strings.ToLower(strings.TrimSpace(change.Artifact.SHA1)); hash != "" {
					live, found = liveBySHA1[hash]
				}
			}
			if !found {
				targetPath := filepath.ToSlash(filepath.Clean(operation.TargetPath))
				if candidate, occupied := liveByPath[targetPath]; occupied &&
					candidate.Management == "managed" &&
					releaseMatchesManagedMod(change.Target, candidate) {
					live, found = candidate, true
				}
			}
			if !found {
				operations = append(operations, operation)
				continue
			}

			name := live.Name
			if strings.TrimSpace(name) == "" {
				name = live.Filename
			}
			plan.Warnings = append(plan.Warnings, Finding{
				Code:         "dependency_already_satisfied",
				CandidateKey: change.CandidateKey,
				Name:         name,
				Path:         live.Path,
				Message: fmt.Sprintf(
					"Provider metadata described %s as an addition, but the exact required managed bytes are already present at %s; no filesystem change is needed for this dependency.",
					name,
					live.Path,
				),
			})
		}
		change.Operations = operations
		if len(change.Operations) == 0 && change.DependencyDriven && !change.Requested {
			continue
		}
		changes = append(changes, change)
	}
	plan.Changes = changes
}

func annotateFindingContexts(plan *Plan, snapshot management.Snapshot) {
	changes := make(map[string]Change, len(plan.Changes))
	for _, change := range plan.Changes {
		changes[change.CandidateKey] = change
	}
	mods := make(map[string]management.Mod, len(snapshot.Mods))
	for _, mod := range snapshot.Mods {
		mods[mod.ID] = mod
		if mod.Provider != "" && mod.ProjectID != "" {
			key := mod.Provider + ":" + mod.ProjectID
			if _, exists := mods[key]; !exists {
				mods[key] = mod
			}
		}
	}

	annotate := func(findings []Finding) {
		for index := range findings {
			finding := &findings[index]
			if change, ok := changes[finding.CandidateKey]; ok {
				if finding.Name == "" {
					finding.Name = change.Name
				}
				if finding.Path == "" {
					finding.Path = findingPathForChange(change)
				}
				continue
			}
			if mod, ok := mods[finding.CandidateKey]; ok {
				if finding.Name == "" {
					finding.Name = mod.Name
				}
				if finding.Path == "" {
					finding.Path = mod.Path
				}
			}
		}
	}
	annotate(plan.Blockers)
	annotate(plan.Warnings)
}

func findingPathForChange(change Change) string {
	for _, operation := range change.Operations {
		if strings.TrimSpace(operation.CurrentPath) != "" {
			return operation.CurrentPath
		}
	}
	for _, operation := range change.Operations {
		if strings.TrimSpace(operation.TargetPath) != "" {
			return operation.TargetPath
		}
	}
	return change.Artifact.Filename
}

func validateOperationCollisions(plan *Plan, mods []management.Mod) {
	liveByPath := make(map[string]management.Mod, len(mods))
	for _, mod := range mods {
		if strings.TrimSpace(mod.Path) != "" {
			liveByPath[filepath.ToSlash(filepath.Clean(mod.Path))] = mod
		}
	}
	plannedTargets := map[string]string{}
	for _, change := range plan.Changes {
		for _, operation := range change.Operations {
			targetPath := filepath.ToSlash(filepath.Clean(operation.TargetPath))
			if ownerKey, exists := plannedTargets[targetPath]; exists && ownerKey != change.CandidateKey {
				addBlocker(
					plan,
					"target_path_conflict",
					change.CandidateKey,
					fmt.Sprintf("Multiple planned artifacts would write %s.", targetPath),
				)
			} else {
				plannedTargets[targetPath] = change.CandidateKey
			}

			live, occupied := liveByPath[targetPath]
			if !occupied {
				continue
			}
			currentPath := filepath.ToSlash(filepath.Clean(operation.CurrentPath))
			if operation.Action == "replace" && currentPath == targetPath {
				continue
			}
			if currentPath != "" && live.Path == operation.CurrentPath {
				continue
			}
			name := live.Name
			if name == "" {
				name = live.Filename
			}
			addBlocker(
				plan,
				"target_path_occupied",
				change.CandidateKey,
				fmt.Sprintf("Planned target %s is already occupied by %s (%s); FPBPack will not overwrite it.", targetPath, name, live.Management),
			)
		}
	}
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
		Summary: fmt.Sprintf("Reviewed %d mod change(s)", len(p.Changes)),
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
