package service

import (
	"testing"
	"time"

	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func TestPendingChangesStageReplacePersistAndDiscard(t *testing.T) {
	now := time.Now().UTC()
	stateDir := t.TempDir()
	svc := &Service{
		options: Options{StateDir: stateDir},
		state: State{
			SchemaVersion: StateSchemaVersion,
			CreatedAt:     now,
			UpdatedAt:     now,
			Settings:      RuntimeSettings{RetentionCount: DefaultRetentionCount},
		},
		updates: updatecheck.Report{
			GeneratedAt: now,
			Minecraft:   "1.21.1",
			Loader:      "neoforge",
			Candidates: []updatecheck.Candidate{{
				Key:            "modrinth:create",
				Provider:       "modrinth",
				ProjectID:      "create",
				Name:           "Create",
				Classification: updatecheck.ClassificationSafe,
				Installed: updatecheck.Release{
					ID:     "v1",
					Number: "1.0.0",
				},
				Target: &updatecheck.Release{
					ID:     "v2",
					Number: "2.0.0",
				},
			}},
		},
		hasUpdate: true,
	}

	pending, err := svc.StagePendingUpdates([]string{"modrinth:create", "modrinth:create"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending.Changes) != 1 {
		t.Fatalf("pending changes = %d, want 1", len(pending.Changes))
	}
	if got := pending.Changes[0].TargetVersion; got != "2.0.0" {
		t.Fatalf("target version = %q, want 2.0.0", got)
	}
	firstRevision := pending.Revision

	// Restaging the same managed artifact replaces its pending intent instead of
	// creating a second operation.
	svc.updates.Candidates[0].Target = &updatecheck.Release{ID: "v3", Number: "3.0.0"}
	pending, err = svc.StagePendingUpdates([]string{"modrinth:create"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending.Changes) != 1 {
		t.Fatalf("restaged pending changes = %d, want 1", len(pending.Changes))
	}
	if got := pending.Changes[0].TargetVersion; got != "3.0.0" {
		t.Fatalf("restaged target version = %q, want 3.0.0", got)
	}
	if pending.Revision <= firstRevision {
		t.Fatalf("revision did not advance: first=%d current=%d", firstRevision, pending.Revision)
	}

	var persisted State
	if err := readJSON(stateDir+"/state.json", &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted.PendingChanges.Changes) != 1 {
		t.Fatalf("persisted pending changes = %d, want 1", len(persisted.PendingChanges.Changes))
	}

	discarded, err := svc.DiscardPendingChanges()
	if err != nil {
		t.Fatal(err)
	}
	if len(discarded.Changes) != 0 {
		t.Fatalf("discarded changes = %d, want 0", len(discarded.Changes))
	}
	if discarded.ReviewedPlanID != "" {
		t.Fatalf("reviewed plan id survived discard: %q", discarded.ReviewedPlanID)
	}
}

func TestUpsertPendingChangeReplacesActionForSameArtifact(t *testing.T) {
	changes := []PendingChange{{
		ID:           "modrinth:create",
		CandidateKey: "modrinth:create",
		Action:       "update",
		Name:         "Create",
		TargetVersion: "2.0.0",
	}}
	changes = upsertPendingChange(changes, PendingChange{
		ID:               "modrinth:create",
		CandidateKey:     "modrinth:create",
		Action:           "remove",
		Name:             "Create",
		InstalledVersion: "1.0.0",
		TargetVersion:    "removed",
	})
	if len(changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(changes))
	}
	if changes[0].Action != "remove" {
		t.Fatalf("action = %q, want remove", changes[0].Action)
	}
}
