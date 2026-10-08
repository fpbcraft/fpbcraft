package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/doctor"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

type ApplyResult struct {
	PlanID      string    `json:"plan_id"`
	BackupID    string    `json:"backup_id"`
	Status      string    `json:"status"`
	AppliedAt   time.Time `json:"applied_at"`
	Mods        int       `json:"mods"`
	Summary     string    `json:"summary"`
}

type RestoreResult struct {
	PlanID      string    `json:"plan_id"`
	BackupID    string    `json:"backup_id"`
	Status      string    `json:"status"`
	RestoredAt  time.Time `json:"restored_at"`
	Mods        int       `json:"mods"`
	Summary     string    `json:"summary"`
}

type stagedPlanOperation struct {
	ChangeName string
	Operation  planning.FileOperation
	StagedPath string
}

func (s *Service) ApplyPlan(ctx context.Context, planID string) (result ApplyResult, err error) {
	defer func() {
		if err != nil {
			s.logEvent("error", "apply", fmt.Sprintf("Apply %s failed: %v", planID, err))
		} else if result.Status != "" {
			s.logEvent("info", "apply", result.Summary)
		}
	}()
	s.logEvent("info", "apply", "Starting reviewed change set "+planID)
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	plan, err := s.Plan(planID)
	if err != nil {
		return ApplyResult{}, err
	}
	if plan.Status != planning.StatusReady || !plan.Verified {
		return ApplyResult{}, fmt.Errorf("reviewed changes %s are not ready and verified", plan.ID)
	}
	if plan.AppliedAt != nil {
		return ApplyResult{}, fmt.Errorf("reviewed changes %s were already applied at %s", plan.ID, plan.AppliedAt.UTC().Format(time.RFC3339))
	}
	if plan.PendingRevision != 0 {
		s.mu.RLock()
		pending := clonePendingChanges(s.state.PendingChanges)
		s.mu.RUnlock()
		if pending.ReviewedPlanID != plan.ID || pending.Revision != plan.PendingRevision {
			return ApplyResult{}, fmt.Errorf("reviewed changes are stale because pending changes were edited or discarded; review the current pending changes again")
		}
	}
	s.logEvent("info", "apply", fmt.Sprintf("Validating %d mod changes and stopped server state", len(plan.Changes)))
	if err := s.requireServerStopped(ctx); err != nil {
		return ApplyResult{}, err
	}
	if err := s.validateCurrentManagedState(); err != nil {
		return ApplyResult{}, fmt.Errorf("live management state is not clean: %w", err)
	}
	if err := s.validatePlanCatalogState(plan); err != nil {
		return ApplyResult{}, fmt.Errorf("reviewed changes no longer match accepted management state: %w", err)
	}

	// Slice 2 may have persisted a ready plan without a restore manifest when
	// only additions were involved. Slice 3 upgrades/creates it immediately
	// before mutation.
	s.logEvent("info", "apply", "Creating restore point for existing JARs")
	if err := s.createRestorePoint(&plan); err != nil {
		return ApplyResult{}, fmt.Errorf("prepare restore point: %w", err)
	}
	if plan.BackupID == "" {
		return ApplyResult{}, fmt.Errorf("reviewed changes have no restore point")
	}
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "plans", plan.ID+".json"), plan); err != nil {
		return ApplyResult{}, fmt.Errorf("persist restore-point link: %w", err)
	}
	rollbackPlan := plan

	manifest, err := s.loadBackupManifest(plan.BackupID)
	if err != nil {
		return ApplyResult{}, err
	}
	if manifest.PlanID != plan.ID {
		return ApplyResult{}, fmt.Errorf("restore point %s belongs to a different change set", manifest.ID)
	}

	s.logEvent("info", "apply", "Checking live files and verified download cache")
	if err := s.validatePlanLiveState(plan); err != nil {
		return ApplyResult{}, fmt.Errorf("reviewed changes are stale: %w", err)
	}
	s.logEvent("info", "apply", "Staging and hashing replacement JARs")
	staged, err := s.stagePlanTargets(plan)
	if err != nil {
		return ApplyResult{}, err
	}
	defer cleanupStagedOperations(staged)

	s.mu.RLock()
	previousState := s.state
	previousSnapshot := s.snapshot
	s.mu.RUnlock()
	committed := false
	defer func() {
		if err == nil || !committed {
			return
		}
		s.logEvent("warn", "apply", "Apply failed after filesystem mutation; restoring original JARs")
		_ = s.rollbackPlanFiles(plan, manifest)
		_ = writeJSONAtomic(filepath.Join(s.options.StateDir, "state.json"), previousState)
		_ = writeJSONAtomic(filepath.Join(s.options.StateDir, "inventory.json"), previousSnapshot.Inventory)
		_ = writeJSONAtomic(filepath.Join(s.options.StateDir, "plans", rollbackPlan.ID+".json"), rollbackPlan)
		s.mu.Lock()
		s.state = previousState
		s.snapshot = previousSnapshot
		s.mu.Unlock()
	}()

	s.logEvent("info", "apply", fmt.Sprintf("Committing %d verified file operations", len(staged)))
	if err = commitStagedOperations(s.options.ServerRoot, staged, func(item stagedPlanOperation) {
		s.logEvent("info", "apply", fmt.Sprintf("%s: %s -> %s", item.ChangeName, item.Operation.Action, item.Operation.TargetPath))
	}); err != nil {
		committed = true
		return ApplyResult{}, fmt.Errorf("apply filesystem changes: %w", err)
	}
	committed = true

	s.logEvent("info", "apply", "File changes committed; rebuilding accepted catalog")
	nextCatalog, err := catalogAfterPlan(previousState.Catalog, plan)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("update accepted catalog: %w", err)
	}
	s.logEvent("info", "apply", "Scanning installed JARs to verify the applied result")
	inv, err := s.scanInventory(ctx)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("verify post-apply inventory: %w", err)
	}
	nextSnapshot := management.BuildSnapshot(inv, nextCatalog)
	if nextSnapshot.Diagnostics.Summary.Blocking > 0 {
		return ApplyResult{}, fmt.Errorf(
			"post-apply verification found %s",
			formatBlockingDiagnostics(nextSnapshot.Diagnostics, 5),
		)
	}

	nextState := previousState
	nextState.Catalog = nextCatalog
	stateChangedAt := time.Now().UTC()
	if nextState.PendingChanges.ReviewedPlanID == plan.ID {
		nextState.PendingChanges = PendingChanges{
			SchemaVersion: PendingChangesSchemaVersion,
			Revision:      nextState.PendingChanges.Revision + 1,
			UpdatedAt:     &stateChangedAt,
			Changes:       []PendingChange{},
		}
	}
	nextState.UpdatedAt = stateChangedAt
	if planTouchesAutoModpack(plan) {
		nextState.AutoModpack.PendingPublish = true
		nextState.AutoModpack.LastChangedAt = &stateChangedAt
	}
	s.logEvent("info", "apply", "Verification passed; persisting inventory and managed state")
	if err = writeJSONAtomic(filepath.Join(s.options.StateDir, "inventory.json"), inv); err != nil {
		return ApplyResult{}, fmt.Errorf("persist post-apply inventory: %w", err)
	}
	if err = writeJSONAtomic(filepath.Join(s.options.StateDir, "state.json"), nextState); err != nil {
		return ApplyResult{}, fmt.Errorf("persist post-apply management state: %w", err)
	}

	now := time.Now().UTC()
	plan.AppliedAt = &now
	if err = writeJSONAtomic(filepath.Join(s.options.StateDir, "plans", plan.ID+".json"), plan); err != nil {
		return ApplyResult{}, fmt.Errorf("persist applied plan: %w", err)
	}

	event := planning.HistoryEvent{
		ID: "apply:" + plan.ID + ":" + now.Format("20060102T150405.000000000Z"),
		CreatedAt: now,
		Type: "apply",
		Status: "success",
		PlanID: plan.ID,
		BackupID: plan.BackupID,
		Mods: len(plan.Changes),
		Summary: fmt.Sprintf("Applied %d mod change(s)", len(plan.Changes)),
	}
	if err = s.persistHistoryEvent(event); err != nil {
		return ApplyResult{}, fmt.Errorf("persist apply history: %w", err)
	}

	nextUpdates := updatecheck.Report{
		GeneratedAt: now,
		Minecraft: s.options.Minecraft,
		Loader: s.options.Loader,
		Candidates: []updatecheck.Candidate{},
	}
	_ = writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), nextUpdates)
	s.mu.Lock()
	s.state = nextState
	s.snapshot = nextSnapshot
	s.updates = nextUpdates
	s.hasUpdate = true
	s.mu.Unlock()

	result = ApplyResult{
		PlanID: plan.ID,
		BackupID: plan.BackupID,
		Status: "success",
		AppliedAt: now,
		Mods: len(plan.Changes),
		Summary: event.Summary,
	}
	return result, nil
}

func (s *Service) RestoreBackup(ctx context.Context, backupID string) (result RestoreResult, err error) {
	defer func() {
		if err != nil {
			s.logEvent("error", "restore", fmt.Sprintf("Restore %s failed: %v", backupID, err))
		} else if result.Status != "" {
			s.logEvent("info", "restore", result.Summary)
		}
	}()
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	s.logEvent("info", "restore", "Starting restore point "+backupID+"; checking stopped server state")
	if err := s.requireServerStopped(ctx); err != nil {
		return RestoreResult{}, err
	}
	manifest, err := s.loadBackupManifest(backupID)
	if err != nil {
		return RestoreResult{}, err
	}
	if manifest.Catalog.SchemaVersion == 0 {
		return RestoreResult{}, fmt.Errorf("restore point %s predates restorable catalog snapshots", backupID)
	}
	plan, err := s.Plan(manifest.PlanID)
	if err != nil {
		return RestoreResult{}, err
	}
	if plan.AppliedAt == nil {
		return RestoreResult{}, fmt.Errorf("plan %s has not been applied", plan.ID)
	}

	if err := s.validateAppliedStateForRestore(plan); err != nil {
		return RestoreResult{}, fmt.Errorf("restore blocked by external change: %w", err)
	}
	s.logEvent("info", "restore", fmt.Sprintf("Verifying %d backed-up JARs", len(manifest.Files)))
	if err := s.validateBackupFiles(manifest); err != nil {
		return RestoreResult{}, err
	}

	s.mu.RLock()
	previousState := s.state
	previousSnapshot := s.snapshot
	s.mu.RUnlock()
	restored := false
	defer func() {
		if err == nil || !restored {
			return
		}
		_ = s.restoreAppliedTargets(plan)
		_ = writeJSONAtomic(filepath.Join(s.options.StateDir, "state.json"), previousState)
		_ = writeJSONAtomic(filepath.Join(s.options.StateDir, "inventory.json"), previousSnapshot.Inventory)
		s.mu.Lock()
		s.state = previousState
		s.snapshot = previousSnapshot
		s.mu.Unlock()
	}()

	s.logEvent("info", "restore", "Restoring verified backup files")
	if err = s.restoreManifestFiles(plan, manifest); err != nil {
		restored = true
		return RestoreResult{}, fmt.Errorf("restore filesystem: %w", err)
	}
	restored = true

	s.logEvent("info", "restore", "Backup files restored; verifying current inventory")
	inv, err := s.scanInventory(ctx)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("verify restored inventory: %w", err)
	}
	nextSnapshot := management.BuildSnapshot(inv, manifest.Catalog)
	if nextSnapshot.Diagnostics.Summary.Blocking > 0 {
		return RestoreResult{}, fmt.Errorf(
			"restored inventory has %s",
			formatBlockingDiagnostics(nextSnapshot.Diagnostics, 5),
		)
	}

	nextState := previousState
	nextState.Catalog = manifest.Catalog
	stateChangedAt := time.Now().UTC()
	nextState.UpdatedAt = stateChangedAt
	if planTouchesAutoModpack(plan) {
		nextState.AutoModpack.PendingPublish = true
		nextState.AutoModpack.LastChangedAt = &stateChangedAt
	}
	if err = writeJSONAtomic(filepath.Join(s.options.StateDir, "inventory.json"), inv); err != nil {
		return RestoreResult{}, err
	}
	if err = writeJSONAtomic(filepath.Join(s.options.StateDir, "state.json"), nextState); err != nil {
		return RestoreResult{}, err
	}

	now := time.Now().UTC()
	event := planning.HistoryEvent{
		ID: "restore:" + backupID + ":" + now.Format("20060102T150405.000000000Z"),
		CreatedAt: now,
		Type: "restore",
		Status: "success",
		PlanID: plan.ID,
		BackupID: backupID,
		Mods: len(plan.Changes),
		Summary: fmt.Sprintf("Restored %d mod change(s) from %s", len(plan.Changes), backupID),
	}
	if err = s.persistHistoryEvent(event); err != nil {
		return RestoreResult{}, err
	}

	nextUpdates := updatecheck.Report{
		GeneratedAt: now,
		Minecraft: s.options.Minecraft,
		Loader: s.options.Loader,
		Candidates: []updatecheck.Candidate{},
	}
	_ = writeJSONAtomic(filepath.Join(s.options.StateDir, "updates.json"), nextUpdates)
	s.mu.Lock()
	s.state = nextState
	s.snapshot = nextSnapshot
	s.updates = nextUpdates
	s.hasUpdate = true
	s.mu.Unlock()

	return RestoreResult{
		PlanID: plan.ID,
		BackupID: backupID,
		Status: "success",
		RestoredAt: now,
		Mods: len(plan.Changes),
		Summary: event.Summary,
	}, nil
}

func (s *Service) loadBackupManifest(backupID string) (planning.BackupManifest, error) {
	if !strings.HasPrefix(backupID, "backup-") || strings.ContainsAny(backupID, "/\\") {
		return planning.BackupManifest{}, fmt.Errorf("invalid backup ID")
	}
	var manifest planning.BackupManifest
	path := filepath.Join(s.options.StateDir, "backups", backupID, "manifest.json")
	if err := readJSON(path, &manifest); err != nil {
		if os.IsNotExist(err) {
			return planning.BackupManifest{}, fmt.Errorf("restore point %s was not found", backupID)
		}
		return planning.BackupManifest{}, err
	}
	return manifest, nil
}

func (s *Service) validateCurrentManagedState() error {
	s.mu.RLock()
	acceptedCatalog := s.state.Catalog
	s.mu.RUnlock()
	inv, err := inventory.Scan(inventory.ScanOptions{
		ServerRoot:     s.options.ServerRoot,
		ServerModsPath: s.options.ServerModsPath,
		ClientModsPath: s.options.ClientModsPath,
	})
	if err != nil {
		return err
	}
	snapshot := management.BuildSnapshot(inv, acceptedCatalog)
	if snapshot.Diagnostics.Summary.Blocking == 0 {
		return nil
	}
	return fmt.Errorf("%s", formatBlockingDiagnostics(snapshot.Diagnostics, 5))
}

func formatBlockingDiagnostics(report doctor.Report, limit int) string {
	if limit < 1 {
		limit = 1
	}
	details := make([]string, 0, limit)
	total := 0
	for _, finding := range report.Findings {
		if finding.Level != doctor.LevelBlocking {
			continue
		}
		total++
		if len(details) >= limit {
			continue
		}

		context := strings.TrimSpace(finding.Mod)
		path := strings.TrimSpace(finding.Path)
		if path != "" {
			if context != "" {
				context += " (" + path + ")"
			} else {
				context = path
			}
		}
		message := strings.TrimSpace(finding.Message)
		if context != "" {
			message = context + ": " + message
		}
		code := strings.TrimSpace(finding.Code)
		if code != "" {
			message = "[" + code + "] " + message
		}
		details = append(details, message)
	}

	if total == 0 {
		total = report.Summary.Blocking
	}
	summary := fmt.Sprintf("%d blocking diagnostic(s)", total)
	if len(details) == 0 {
		return summary
	}
	summary += ": " + strings.Join(details, "; ")
	if total > len(details) {
		summary += fmt.Sprintf("; +%d more", total-len(details))
	}
	return summary
}

func (s *Service) validatePlanCatalogState(plan planning.Plan) error {
	s.mu.RLock()
	accepted := append([]catalog.Entry(nil), s.state.Catalog.Managed...)
	s.mu.RUnlock()
	managed := make(map[string]catalog.Entry, len(accepted))
	for _, entry := range accepted {
		managed[catalog.EntryKey(entry)] = entry
	}
	for _, change := range plan.Changes {
		for _, operation := range change.Operations {
			entry, exists := managed[change.CandidateKey]
			switch operation.Action {
			case "replace", "remove":
				if !exists {
					return fmt.Errorf("%s is no longer a managed artifact", change.Name)
				}
				if operation.CurrentSHA512 != "" &&
					!strings.EqualFold(entry.SHA512, operation.CurrentSHA512) {
					return fmt.Errorf("%s accepted artifact hash changed after these changes were reviewed", change.Name)
				}
				if operation.CurrentPath != "" && !sourcesContainPath(entry.SourcePaths, operation.CurrentPath) {
					return fmt.Errorf("%s accepted source path changed after these changes were reviewed", change.Name)
				}
				if operation.Action == "replace" {
					if expected := inventory.Location(change.Artifact.Deployment); expected != "" && entry.Deployment != expected {
						return fmt.Errorf(
							"%s preferred placement changed from %s to %s after these changes were reviewed",
							change.Name,
							expected,
							entry.Deployment,
						)
					}
					if entry.Deployment == inventory.LocationClient {
						expectedGroup := normalizeAutoModpackGroup(inventory.LocationClient, change.Artifact.AutoModpackGroup)
						currentGroup := normalizeAutoModpackGroup(inventory.LocationClient, entry.AutoModpackGroup)
						if expectedGroup != currentGroup {
							return fmt.Errorf(
								"%s preferred AutoModpack group changed from %s to %s after these changes were reviewed",
								change.Name,
								expectedGroup,
								currentGroup,
							)
						}
					}
				}
			case "add":
				if exists {
					return fmt.Errorf("%s is now already managed; review the pending changes again", change.Name)
				}
			default:
				return fmt.Errorf("%s has unsupported operation %q", change.Name, operation.Action)
			}
		}
	}
	return nil
}

func (s *Service) validatePlanLiveState(plan planning.Plan) error {
	cacheByHash := s.prefetchedByHash(plan)
	for _, change := range plan.Changes {
		for _, operation := range change.Operations {
			targetRel, err := safeRelativePath(operation.TargetPath)
			if err != nil {
				return err
			}
			target := filepath.Join(s.options.ServerRoot, targetRel)

			if operation.Action == "replace" || operation.Action == "remove" {
				currentRel, err := safeRelativePath(operation.CurrentPath)
				if err != nil {
					return err
				}
				current := filepath.Join(s.options.ServerRoot, currentRel)
				hash, err := sha512File(current)
				if err != nil {
					return fmt.Errorf("%s current artifact %s: %w", change.Name, operation.CurrentPath, err)
				}
				if operation.CurrentSHA512 != "" && !strings.EqualFold(hash, operation.CurrentSHA512) {
					return fmt.Errorf("%s current artifact changed: %s", change.Name, operation.CurrentPath)
				}
				if operation.Action == "replace" && currentRel != targetRel {
					if _, err := os.Stat(target); err == nil {
						return fmt.Errorf("%s target path is now occupied: %s", change.Name, operation.TargetPath)
					} else if !os.IsNotExist(err) {
						return err
					}
				}
			} else if operation.Action == "add" {
				if _, err := os.Stat(target); err == nil {
					return fmt.Errorf("%s target path is now occupied: %s", change.Name, operation.TargetPath)
				} else if !os.IsNotExist(err) {
					return err
				}
			} else {
				return fmt.Errorf("unsupported plan operation %q", operation.Action)
			}

			if operation.Action == "remove" {
				continue
			}
			cachePath, ok := cacheByHash[strings.ToLower(operation.TargetSHA512)]
			if !ok {
				return fmt.Errorf("%s verified cached artifact is missing from the reviewed changes", change.Name)
			}
			hash, err := sha512File(cachePath)
			if err != nil {
				return fmt.Errorf("%s cached artifact: %w", change.Name, err)
			}
			if !strings.EqualFold(hash, operation.TargetSHA512) {
				return fmt.Errorf("%s cached artifact hash changed", change.Name)
			}
		}
	}
	return nil
}

func (s *Service) stagePlanTargets(plan planning.Plan) ([]stagedPlanOperation, error) {
	cacheByHash := s.prefetchedByHash(plan)
	staged := make([]stagedPlanOperation, 0)
	for _, change := range plan.Changes {
		for _, operation := range change.Operations {
			if operation.Action == "remove" {
				staged = append(staged, stagedPlanOperation{
					ChangeName: change.Name,
					Operation: operation,
				})
				continue
			}
			targetRel, err := safeRelativePath(operation.TargetPath)
			if err != nil {
				cleanupStagedOperations(staged)
				return nil, err
			}
			target := filepath.Join(s.options.ServerRoot, targetRel)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				cleanupStagedOperations(staged)
				return nil, err
			}
			tmp, err := os.CreateTemp(filepath.Dir(target), ".fpbpack-apply-*")
			if err != nil {
				cleanupStagedOperations(staged)
				return nil, err
			}
			stagePath := tmp.Name()
			if err := tmp.Close(); err != nil {
				cleanupStagedOperations(staged)
				return nil, err
			}
			_ = os.Remove(stagePath)

			cachePath := cacheByHash[strings.ToLower(operation.TargetSHA512)]
			_, hash, err := copyFileWithSHA512(cachePath, stagePath)
			if err != nil {
				cleanupStagedOperations(staged)
				return nil, fmt.Errorf("stage %s: %w", change.Name, err)
			}
			if !strings.EqualFold(hash, operation.TargetSHA512) {
				_ = os.Remove(stagePath)
				cleanupStagedOperations(staged)
				return nil, fmt.Errorf("stage %s: SHA-512 mismatch", change.Name)
			}
			staged = append(staged, stagedPlanOperation{
				ChangeName: change.Name,
				Operation: operation,
				StagedPath: stagePath,
			})
		}
	}
	return staged, nil
}

func commitStagedOperations(serverRoot string, staged []stagedPlanOperation, onComplete func(stagedPlanOperation)) error {
	for index := range staged {
		item := &staged[index]
		targetRel, err := safeRelativePath(item.Operation.TargetPath)
		if err != nil {
			return err
		}
		target := filepath.Join(serverRoot, targetRel)
		if item.Operation.Action == "remove" {
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("%s: remove %s: %w", item.ChangeName, item.Operation.TargetPath, err)
			}
			if onComplete != nil {
				onComplete(*item)
			}
			continue
		}
		if err := os.Rename(item.StagedPath, target); err != nil {
			return fmt.Errorf("%s: install %s: %w", item.ChangeName, item.Operation.TargetPath, err)
		}
		item.StagedPath = ""
		if item.Operation.Action == "replace" {
			currentRel, err := safeRelativePath(item.Operation.CurrentPath)
			if err != nil {
				return err
			}
			if currentRel != targetRel {
				if err := os.Remove(filepath.Join(serverRoot, currentRel)); err != nil {
					return fmt.Errorf("%s: remove old artifact %s: %w", item.ChangeName, item.Operation.CurrentPath, err)
				}
			}
		}
		if onComplete != nil {
			onComplete(*item)
		}
	}
	return nil
}

func cleanupStagedOperations(staged []stagedPlanOperation) {
	for _, item := range staged {
		if item.StagedPath != "" {
			_ = os.Remove(item.StagedPath)
		}
	}
}

func prefetchedByHash(plan planning.Plan) map[string]string {
	result := make(map[string]string, len(plan.Prefetched))
	for _, artifact := range plan.Prefetched {
		relative, err := safeRelativePath(artifact.CachePath)
		if err != nil {
			continue
		}
		result[strings.ToLower(artifact.SHA512)] = relative
	}
	return result
}

func (s *Service) prefetchedByHash(plan planning.Plan) map[string]string {
	result := prefetchedByHash(plan)
	for hash, relative := range result {
		result[hash] = filepath.Join(s.options.StateDir, relative)
	}
	return result
}

func (s *Service) rollbackPlanFiles(plan planning.Plan, manifest planning.BackupManifest) error {
	for _, change := range plan.Changes {
		for _, operation := range change.Operations {
			relative, err := safeRelativePath(operation.TargetPath)
			if err != nil {
				continue
			}
			_ = os.Remove(filepath.Join(s.options.ServerRoot, relative))
		}
	}
	for _, file := range manifest.Files {
		sourceRel, err := safeRelativePath(file.SourcePath)
		if err != nil {
			return err
		}
		backupRel, err := safeRelativePath(file.BackupPath)
		if err != nil {
			return err
		}
		source := filepath.Join(s.options.ServerRoot, sourceRel)
		backup := filepath.Join(s.options.StateDir, "backups", manifest.ID, backupRel)
		_, hash, err := copyFileWithSHA512(backup, source)
		if err != nil {
			return err
		}
		if !strings.EqualFold(hash, file.SHA512) {
			return fmt.Errorf("rollback hash mismatch for %s", file.SourcePath)
		}
	}
	return nil
}

func (s *Service) validateBackupFiles(manifest planning.BackupManifest) error {
	for _, file := range manifest.Files {
		relative, err := safeRelativePath(file.BackupPath)
		if err != nil {
			return err
		}
		path := filepath.Join(s.options.StateDir, "backups", manifest.ID, relative)
		hash, err := sha512File(path)
		if err != nil {
			return fmt.Errorf("restore point file %s: %w", file.BackupPath, err)
		}
		if !strings.EqualFold(hash, file.SHA512) {
			return fmt.Errorf("restore point file changed: %s", file.BackupPath)
		}
	}
	return nil
}

func (s *Service) validateAppliedStateForRestore(plan planning.Plan) error {
	for _, change := range plan.Changes {
		for _, operation := range change.Operations {
			relative, err := safeRelativePath(operation.TargetPath)
			if err != nil {
				return err
			}
			path := filepath.Join(s.options.ServerRoot, relative)
			if operation.Action == "remove" {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("%s was recreated after the removal was applied", operation.TargetPath)
				} else if !os.IsNotExist(err) {
					return err
				}
				continue
			}
			hash, err := sha512File(path)
			if err != nil {
				return fmt.Errorf("%s: %w", operation.TargetPath, err)
			}
			if !strings.EqualFold(hash, operation.TargetSHA512) {
				return fmt.Errorf("%s no longer matches the applied change set", operation.TargetPath)
			}
		}
	}
	return nil
}

func (s *Service) restoreManifestFiles(plan planning.Plan, manifest planning.BackupManifest) error {
	type stagedBackup struct {
		sourceRel string
		stagePath string
		hash string
	}
	staged := make([]stagedBackup, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		sourceRel, err := safeRelativePath(file.SourcePath)
		if err != nil {
			return err
		}
		backupRel, err := safeRelativePath(file.BackupPath)
		if err != nil {
			return err
		}
		source := filepath.Join(s.options.ServerRoot, sourceRel)
		if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
			return err
		}
		tmp, err := os.CreateTemp(filepath.Dir(source), ".fpbpack-restore-*")
		if err != nil {
			return err
		}
		stagePath := tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(stagePath)
		_, hash, err := copyFileWithSHA512(
			filepath.Join(s.options.StateDir, "backups", manifest.ID, backupRel),
			stagePath,
		)
		if err != nil {
			return err
		}
		if !strings.EqualFold(hash, file.SHA512) {
			return fmt.Errorf("staged restore hash mismatch for %s", file.SourcePath)
		}
		staged = append(staged, stagedBackup{sourceRel: sourceRel, stagePath: stagePath, hash: hash})
	}
	defer func() {
		for _, file := range staged {
			if file.stagePath != "" {
				_ = os.Remove(file.stagePath)
			}
		}
	}()

	for _, change := range plan.Changes {
		for _, operation := range change.Operations {
			targetRel, err := safeRelativePath(operation.TargetPath)
			if err != nil {
				return err
			}
			_ = os.Remove(filepath.Join(s.options.ServerRoot, targetRel))
		}
	}
	for index := range staged {
		source := filepath.Join(s.options.ServerRoot, staged[index].sourceRel)
		if err := os.Rename(staged[index].stagePath, source); err != nil {
			return err
		}
		staged[index].stagePath = ""
	}
	return nil
}

func (s *Service) restoreAppliedTargets(plan planning.Plan) error {
	cacheByHash := s.prefetchedByHash(plan)
	for _, change := range plan.Changes {
		for _, operation := range change.Operations {
			sourceRel, _ := safeRelativePath(operation.CurrentPath)
			if sourceRel != "" {
				_ = os.Remove(filepath.Join(s.options.ServerRoot, sourceRel))
			}
			targetRel, err := safeRelativePath(operation.TargetPath)
			if err != nil {
				return err
			}
			target := filepath.Join(s.options.ServerRoot, targetRel)
			if operation.Action == "remove" {
				_ = os.Remove(target)
				continue
			}
			cache, ok := cacheByHash[strings.ToLower(operation.TargetSHA512)]
			if !ok {
				return fmt.Errorf("cached applied artifact is unavailable for %s", change.Name)
			}
			_, hash, err := copyFileWithSHA512(cache, target)
			if err != nil {
				return err
			}
			if !strings.EqualFold(hash, operation.TargetSHA512) {
				return fmt.Errorf("reapply hash mismatch for %s", change.Name)
			}
		}
	}
	return nil
}

func catalogAfterPlan(current catalog.Report, plan planning.Plan) (catalog.Report, error) {
	bytes, err := json.Marshal(current)
	if err != nil {
		return catalog.Report{}, err
	}
	var next catalog.Report
	if err := json.Unmarshal(bytes, &next); err != nil {
		return catalog.Report{}, err
	}

	for _, change := range plan.Changes {
		found := -1
		for index, entry := range next.Managed {
			if catalog.EntryKey(entry) == change.CandidateKey {
				found = index
				break
			}
		}

		isRemoval := false
		for _, operation := range change.Operations {
			if operation.Action == "remove" {
				isRemoval = true
				break
			}
		}
		if isRemoval {
			if found >= 0 {
				next.Managed = append(next.Managed[:found], next.Managed[found+1:]...)
			}
			continue
		}

		entry := catalog.Entry{
			Provider: change.Artifact.Provider,
			ProjectID: change.Artifact.ProjectID,
			VersionID: change.Target.ID,
			Name: change.Name,
			Filename: change.Artifact.Filename,
			SHA1: change.Artifact.SHA1,
			SHA512: change.Artifact.SHA512,
			URL: change.Artifact.URL,
			Deployment: inventory.Location(change.Artifact.Deployment),
			AutoModpackGroup: normalizeAutoModpackGroup(inventory.Location(change.Artifact.Deployment), change.Artifact.AutoModpackGroup),
			Environment: change.Artifact.Environment,
		}
		if entry.Deployment == inventory.LocationClient {
			entry.Side = "client"
		} else {
			entry.Side = "both"
		}
		if found >= 0 {
			previous := next.Managed[found]
			entry.ArtifactID = previous.ArtifactID
			entry.Side = previous.Side
			if strings.TrimSpace(entry.Environment) == "" {
				entry.Environment = previous.Environment
			}
			entry.Repository = previous.Repository
		}
		switch entry.Provider {
		case "github":
			entry.Repository = change.Artifact.ProjectID
			entry.Tag = change.Target.ID
			entry.Asset = change.Artifact.Filename
		case "curseforge":
			value, parseErr := strconv.ParseUint(change.Target.ID, 10, 32)
			if parseErr != nil {
				return catalog.Report{}, fmt.Errorf("%s target file ID is invalid: %w", change.Name, parseErr)
			}
			entry.FileID = uint32(value)
		}
		for _, operation := range change.Operations {
			entry.SourcePaths = append(entry.SourcePaths, catalog.Source{
				Location: entry.Deployment,
				Group: entry.AutoModpackGroup,
				Path: filepath.ToSlash(operation.TargetPath),
			})
		}

		if found >= 0 {
			next.Managed[found] = entry
		} else {
			next.Managed = append(next.Managed, entry)
		}
	}
	catalog.EnsureManagedArtifactIDs(next.Managed)
	next.RecalculateSummary()
	return next, nil
}

func (s *Service) persistHistoryEvent(event planning.HistoryEvent) error {
	path := filepath.Join(
		s.options.StateDir,
		"history",
		event.CreatedAt.UTC().Format("20060102T150405.000000000Z")+"-"+strings.ReplaceAll(event.ID, ":", "-")+".json",
	)
	if err := writeJSONAtomic(path, event); err != nil {
		return err
	}
	return s.pruneHistory(s.Settings().RetentionCount)
}


func planTouchesAutoModpack(plan planning.Plan) bool {
	for _, change := range plan.Changes {
		if inventory.Location(change.Artifact.Deployment) == inventory.LocationClient {
			return true
		}
		for _, operation := range change.Operations {
			for _, path := range []string{operation.CurrentPath, operation.TargetPath} {
				path = filepath.ToSlash(filepath.Clean(path))
				if strings.HasPrefix(path, inventory.DefaultAutoModpackHostPath+"/") {
					return true
				}
			}
		}
	}
	return false
}
