package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
)

func TestUpdateSettingsPersistsRetention(t *testing.T) {
	stateDir := t.TempDir()
	s := &Service{
		options: Options{StateDir: stateDir},
		state: State{
			SchemaVersion: StateSchemaVersion,
			CreatedAt: time.Now().UTC(),
			Settings: RuntimeSettings{RetentionCount: DefaultRetentionCount},
		},
	}
	if err := s.persistState(); err != nil {
		t.Fatal(err)
	}
	got, err := s.UpdateSettings(RuntimeSettings{RetentionCount: 7})
	if err != nil {
		t.Fatal(err)
	}
	if got.RetentionCount != 7 {
		t.Fatalf("retention = %d", got.RetentionCount)
	}
	var persisted State
	bytes, err := os.ReadFile(filepath.Join(stateDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bytes, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Settings.RetentionCount != 7 {
		t.Fatalf("persisted retention = %d", persisted.Settings.RetentionCount)
	}
}

func TestUpdateSettingsValidatesRetention(t *testing.T) {
	s := &Service{options: Options{StateDir: t.TempDir()}}
	for _, value := range []int{0, 101} {
		if _, err := s.UpdateSettings(RuntimeSettings{RetentionCount: value}); err == nil {
			t.Fatalf("expected retention %d to fail", value)
		}
	}
}

func TestPruneHistoryRemovesOldPlanAndBackup(t *testing.T) {
	stateDir := t.TempDir()
	s := &Service{options: Options{StateDir: stateDir}}
	ids := []string{"0000000000000001", "0000000000000002", "0000000000000003"}
	for i, suffix := range ids {
		planID := "plan-" + suffix
		backupID := "backup-" + suffix
		created := time.Date(2026, 10, 5, 12, i, 0, 0, time.UTC)
		if err := writeJSONAtomic(filepath.Join(stateDir, "plans", planID+".json"), planning.Plan{ID: planID, CreatedAt: created}); err != nil {
			t.Fatal(err)
		}
		if err := writeJSONAtomic(filepath.Join(stateDir, "backups", backupID, "manifest.json"), planning.BackupManifest{ID: backupID, PlanID: planID}); err != nil {
			t.Fatal(err)
		}
		event := planning.HistoryEvent{
			ID: "plan:" + planID, Type: "plan", PlanID: planID, BackupID: backupID, CreatedAt: created,
		}
		if err := writeJSONAtomic(filepath.Join(stateDir, "history", created.Format("20060102T150405")+"-"+planID+".json"), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.pruneHistory(2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "plans", "plan-0000000000000001.json")); !os.IsNotExist(err) {
		t.Fatalf("oldest plan was not pruned")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "backups", "backup-0000000000000001")); !os.IsNotExist(err) {
		t.Fatalf("oldest backup was not pruned")
	}
	history, err := s.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history length = %d, want 2", len(history))
	}
}
