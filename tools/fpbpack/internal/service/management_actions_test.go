package service

import (
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
)

func TestMarkModUnmanagedClearsUnresolvedBlocker(t *testing.T) {
	now := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	mod := inventory.ModFile{
		Location: inventory.LocationServer,
		Path: "mods/custom.jar",
		Filename: "custom.jar",
		SHA1: "sha1",
		SHA512: "sha512",
	}
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		GeneratedAt: now,
		ModrinthChecked: true,
		Mods: []inventory.ModFile{mod},
	}
	cat := catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		InventorySchema: inventory.SchemaVersion,
		Unresolved: []catalog.Unresolved{{
			SHA512: mod.SHA512,
			Filename: mod.Filename,
			Sources: []catalog.Source{{Location: mod.Location, Path: mod.Path}},
		}},
	}
	cat.RecalculateSummary()

	s := &Service{
		options: Options{StateDir: t.TempDir()},
		state: State{
			SchemaVersion: StateSchemaVersion,
			CreatedAt: now,
			UpdatedAt: now,
			Settings: RuntimeSettings{RetentionCount: DefaultRetentionCount},
			Catalog: cat,
		},
		snapshot: management.BuildSnapshot(inv, cat),
	}

	if s.snapshot.Diagnostics.Summary.Blocking == 0 {
		t.Fatal("fixture should begin with a blocking unresolved artifact")
	}
	result, err := s.markModUnmanaged(mod.Path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Management != "unmanaged" {
		t.Fatalf("management = %q", result.Management)
	}
	if len(s.state.Catalog.Unresolved) != 0 || len(s.state.Catalog.Pinned) != 1 {
		t.Fatalf("catalog was not moved to pinned: %+v", s.state.Catalog)
	}
	if s.snapshot.Diagnostics.Summary.Blocking != 0 {
		t.Fatalf("blocking diagnostics remain: %+v", s.snapshot.Diagnostics.Findings)
	}
}

func TestForgetMissingAcceptedEntryClearsMissingBlocker(t *testing.T) {
	now := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		GeneratedAt: now,
		ModrinthChecked: true,
	}
	cat := catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		InventorySchema: inventory.SchemaVersion,
		Managed: []catalog.Entry{{
			Provider: "modrinth",
			ProjectID: "missing",
			Name: "Missing",
			Filename: "missing.jar",
			SHA512: "missing-hash",
			Deployment: inventory.LocationServer,
			SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/missing.jar"}},
		}},
	}
	cat.RecalculateSummary()

	s := &Service{
		options: Options{StateDir: t.TempDir()},
		state: State{
			SchemaVersion: StateSchemaVersion,
			CreatedAt: now,
			UpdatedAt: now,
			Settings: RuntimeSettings{RetentionCount: DefaultRetentionCount},
			Catalog: cat,
		},
		snapshot: management.BuildSnapshot(inv, cat),
	}
	if s.snapshot.Diagnostics.Summary.Blocking == 0 {
		t.Fatal("fixture should begin with a blocking missing artifact")
	}
	if _, err := s.forgetMissingAcceptedEntry("mods/missing.jar"); err != nil {
		t.Fatal(err)
	}
	if len(s.state.Catalog.Managed) != 0 {
		t.Fatalf("managed entry still exists: %+v", s.state.Catalog.Managed)
	}
	if s.snapshot.Diagnostics.Summary.Blocking != 0 {
		t.Fatalf("blocking diagnostics remain: %+v", s.snapshot.Diagnostics.Findings)
	}
}


func TestReplaceCatalogArtifactPreservesSiblingFromSameRepository(t *testing.T) {
	now := time.Now().UTC()
	recorder := inventory.ModFile{
		Location: inventory.LocationServer, Path: "mods/recorder.jar",
		Filename: "recorder.jar", SHA512: "recorder",
	}
	bluemap := inventory.ModFile{
		Location: inventory.LocationServer, Path: "mods/bluemap.jar",
		Filename: "bluemap.jar", SHA512: "bluemap",
	}
	cat := catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		InventorySchema: inventory.SchemaVersion,
		Managed: []catalog.Entry{{
			Provider: "github", ProjectID: "fpbcraft/player-history-mc",
			Name: "Recorder", Filename: recorder.Filename, SHA512: recorder.SHA512,
			Deployment: inventory.LocationServer,
			SourcePaths: []catalog.Source{{Location: recorder.Location, Path: recorder.Path}},
		}},
		Unresolved: []catalog.Unresolved{{
			SHA512: bluemap.SHA512, Filename: bluemap.Filename,
			Sources: []catalog.Source{{Location: bluemap.Location, Path: bluemap.Path}},
		}},
	}
	cat.RecalculateSummary()
	s := &Service{
		options: Options{StateDir: t.TempDir()},
		state: State{
			SchemaVersion: StateSchemaVersion, CreatedAt: now, UpdatedAt: now,
			Settings: RuntimeSettings{RetentionCount: DefaultRetentionCount}, Catalog: cat,
		},
		snapshot: management.BuildSnapshot(inventory.Inventory{
			SchemaVersion: inventory.SchemaVersion, GeneratedAt: now,
			ModrinthChecked: true, Mods: []inventory.ModFile{recorder, bluemap},
		}, cat),
	}

	s.replaceCatalogArtifact(bluemap, catalog.Entry{
		Provider: "github", ProjectID: "fpbcraft/player-history-mc",
		Name: "BlueMap addon", Filename: bluemap.Filename, SHA512: bluemap.SHA512,
		Deployment: inventory.LocationServer,
		SourcePaths: []catalog.Source{{Location: bluemap.Location, Path: bluemap.Path}},
	})
	if err := s.persistCatalogMutation(); err != nil {
		t.Fatal(err)
	}
	if len(s.state.Catalog.Managed) != 2 {
		t.Fatalf("managed entries = %d, want 2: %+v", len(s.state.Catalog.Managed), s.state.Catalog.Managed)
	}
	if catalog.EntryKey(s.state.Catalog.Managed[0]) == catalog.EntryKey(s.state.Catalog.Managed[1]) {
		t.Fatalf("sibling entries still collide: %+v", s.state.Catalog.Managed)
	}
}

func TestSetPreferredPlacementDoesNotMoveLiveArtifact(t *testing.T) {
	now := time.Now().UTC()
	mod := inventory.ModFile{
		Location: inventory.LocationServer, Path: "mods/test.jar",
		Filename: "test.jar", SHA512: "test",
	}
	cat := catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		InventorySchema: inventory.SchemaVersion,
		Managed: []catalog.Entry{{
			Provider: "modrinth", ProjectID: "test", Name: "Test",
			Filename: mod.Filename, SHA512: mod.SHA512,
			Deployment: inventory.LocationServer,
			SourcePaths: []catalog.Source{{Location: mod.Location, Path: mod.Path}},
		}},
	}
	cat.RecalculateSummary()
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion, GeneratedAt: now,
		ModrinthChecked: true, Mods: []inventory.ModFile{mod},
	}
	s := &Service{
		options: Options{StateDir: t.TempDir()},
		state: State{
			SchemaVersion: StateSchemaVersion, CreatedAt: now, UpdatedAt: now,
			Settings: RuntimeSettings{RetentionCount: DefaultRetentionCount}, Catalog: cat,
		},
		snapshot: management.BuildSnapshot(inv, cat),
	}

	if _, err := s.setPreferredPlacement(mod.Path, "client"); err != nil {
		t.Fatal(err)
	}
	if got := s.state.Catalog.Managed[0].Deployment; got != inventory.LocationClient {
		t.Fatalf("preferred deployment = %q", got)
	}
	if got := s.snapshot.Mods[0].Deployment; got != inventory.LocationServer {
		t.Fatalf("current deployment changed in snapshot: %q", got)
	}
	if got := s.snapshot.Mods[0].PreferredDeployment; got != inventory.LocationClient {
		t.Fatalf("preferred deployment in snapshot = %q", got)
	}
}
