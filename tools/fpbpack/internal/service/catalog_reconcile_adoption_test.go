package service

import (
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestAdoptExactInventoryMatchesPreservesDualPlacement(t *testing.T) {
	hash := "sparkweave-sha512"
	snapshot := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		ModrinthChecked: true,
		Mods: []inventory.ModFile{
			{
				Location: inventory.LocationServer,
				Path: "mods/Sparkweave.jar",
				Filename: "Sparkweave.jar",
				SHA512: hash,
				Modrinth: &inventory.ModrinthMatch{
					ProjectID: "sparkweave", VersionID: "v1",
					URL: "https://cdn.modrinth.com/sparkweave.jar",
				},
			},
			{
				Location: inventory.LocationClient, Group: "main",
				Path: "automodpack/host-modpack/main/mods/Sparkweave.jar",
				Filename: "Sparkweave.jar",
				SHA512: hash,
				Modrinth: &inventory.ModrinthMatch{
					ProjectID: "sparkweave", VersionID: "v1",
					URL: "https://cdn.modrinth.com/sparkweave.jar",
				},
			},
		},
	}
	report := catalog.Report{SchemaVersion: catalog.ReportSchemaVersion, InventorySchema: inventory.SchemaVersion}
	if count := adoptExactInventoryMatches(snapshot, &report); count != 1 {
		t.Fatalf("adopted %d, want one content identity", count)
	}
	if len(report.Managed) != 1 || len(report.Managed[0].SourcePaths) != 2 {
		t.Fatalf("two placements should share one managed identity: %+v", report.Managed)
	}
	if count := adoptExactInventoryMatches(snapshot, &report); count != 0 {
		t.Fatalf("second adoption should be idempotent: %d", count)
	}
}

func TestAdoptExactInventoryMatchesRefusesLiveConflictingVersion(t *testing.T) {
	report := catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		InventorySchema: inventory.SchemaVersion,
		Managed: []catalog.Entry{{
			Provider: "modrinth", ProjectID: "sparkweave", VersionID: "old",
			SHA512: "old-hash",
			SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/old.jar"}},
		}},
	}
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion, ModrinthChecked: true,
		Mods: []inventory.ModFile{
			{Location: inventory.LocationServer, Path: "mods/old.jar", SHA512: "old-hash"},
			{Location: inventory.LocationServer, Path: "mods/new.jar", SHA512: "new-hash",
				Modrinth: &inventory.ModrinthMatch{ProjectID: "sparkweave", VersionID: "new",
					URL: "https://cdn.modrinth.com/new.jar"}},
		},
	}
	if count := adoptExactInventoryMatches(inv, &report); count != 0 {
		t.Fatalf("should not silently adopt a duplicate version: %d", count)
	}
	if len(report.Managed) != 1 || report.Managed[0].SHA512 != "old-hash" {
		t.Fatalf("existing managed identity changed: %+v", report.Managed)
	}
}
