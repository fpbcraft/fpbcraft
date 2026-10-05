package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
)

func TestValidPlanID(t *testing.T) {
	if !validPlanID("plan-0123456789abcdef") {
		t.Fatal("valid plan id rejected")
	}
	for _, value := range []string{"../plan-x", "plan-short", "other-0123456789abcdef"} {
		if validPlanID(value) {
			t.Fatalf("invalid plan id accepted: %q", value)
		}
	}
}

func TestPlansAndHistoryReturnEmptyForFreshState(t *testing.T) {
	service := &Service{options: Options{StateDir: t.TempDir()}}
	plans, err := service.Plans()
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 0 {
		t.Fatalf("plans = %d, want 0", len(plans))
	}
	history, err := service.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("history = %d, want 0", len(history))
	}
	if _, err := os.Stat(filepath.Join(service.options.StateDir, "plans")); !os.IsNotExist(err) {
		t.Fatalf("reading plans should not create directories")
	}
}

func TestPersistedPlanReadyRequiresRestorePoint(t *testing.T) {
	stateDir := t.TempDir()
	s := &Service{options: Options{StateDir: stateDir}}
	plan := planning.Plan{
		ID: "plan-0123456789abcdef",
		Status: planning.StatusReady,
		Verified: true,
		RequiresBackup: true,
		BackupID: "backup-0123456789abcdef",
	}
	if s.persistedPlanReady(plan) {
		t.Fatal("plan without backup manifest must not be reusable")
	}
	manifest := planning.BackupManifest{
		ID: plan.BackupID,
		PlanID: plan.ID,
		Files: []planning.BackupFile{{SourcePath: "mods/a.jar", BackupPath: "files/mods/a.jar"}},
	}
	if err := writeJSONAtomic(filepath.Join(stateDir, "backups", plan.BackupID, "manifest.json"), manifest); err != nil {
		t.Fatal(err)
	}
	if !s.persistedPlanReady(plan) {
		t.Fatal("verified plan with restore point should be reusable")
	}
}
