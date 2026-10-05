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
