package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
)

func TestNewBootstrapsAndOwnsGeneratedCaches(t *testing.T) {
	root := t.TempDir()
	stateDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}

	service, err := New(context.Background(), Options{
		ServerRoot: root,
		StateDir:   stateDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"state.json", "inventory.json", "updates.json"} {
		if _, err := os.Stat(filepath.Join(stateDir, name)); err != nil {
			t.Fatalf("%s was not generated: %v", name, err)
		}
	}
	snapshot, err := service.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status.Mods != 0 {
		t.Fatalf("mods = %d, want 0", snapshot.Status.Mods)
	}
	if _, err := service.Updates(); err != nil {
		t.Fatalf("updates not ready: %v", err)
	}
}

func TestNewImportsLegacyReportOnlyWhenStateIsMissing(t *testing.T) {
	root := t.TempDir()
	stateDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}

	report := catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		Pinned: []catalog.PinnedArtifact{{
			SHA512: "legacy",
			Filename: "custom.jar",
			Reason: "legacy unmanaged artifact",
		}},
	}
	report.Summary.PinnedArtifacts = 1
	bytes, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(stateDir, "migration-report.json")
	if err := os.WriteFile(legacyPath, bytes, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := New(context.Background(), Options{ServerRoot: root, StateDir: stateDir}); err != nil {
		t.Fatal(err)
	}

	var state State
	stateBytes, err := os.ReadFile(filepath.Join(stateDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		t.Fatal(err)
	}
	if state.ImportedFrom == "" {
		t.Fatal("expected imported_from to record legacy bootstrap")
	}
	if len(state.Catalog.Pinned) != 1 {
		t.Fatalf("pinned = %d, want 1", len(state.Catalog.Pinned))
	}
}
