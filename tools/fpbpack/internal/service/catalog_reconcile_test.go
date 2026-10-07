package service

import (
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestReconcileManagedSourcePathsAdoptsManualMoveByExactHash(t *testing.T) {
	report := catalog.Report{Managed: []catalog.Entry{{
		Provider: "modrinth",
		ProjectID: "wavify",
		Name: "Wavify",
		SHA512: "same-bytes",
		Deployment: inventory.LocationClient,
		AutoModpackGroup: "main",
		SourcePaths: []catalog.Source{{
			Location: inventory.LocationClient,
			Group: "main",
			Path: "automodpack/host-modpack/main/mods/wavify.jar",
		}},
	}}}
	inv := inventory.Inventory{Mods: []inventory.ModFile{{
		Location: inventory.LocationClient,
		Group: "visual-client-mods",
		Path: "automodpack/host-modpack/visual-client-mods/mods/wavify.jar",
		SHA512: "same-bytes",
	}}}

	if changed := reconcileManagedSourcePaths(inv, &report); changed != 1 {
		t.Fatalf("changed = %d, want 1", changed)
	}
	entry := report.Managed[0]
	if entry.Deployment != inventory.LocationClient || entry.AutoModpackGroup != "main" {
		t.Fatalf("preferred placement changed unexpectedly: %+v", entry)
	}
	if len(entry.SourcePaths) != 1 ||
		entry.SourcePaths[0].Group != "visual-client-mods" ||
		entry.SourcePaths[0].Path != "automodpack/host-modpack/visual-client-mods/mods/wavify.jar" {
		t.Fatalf("live source was not reconciled: %+v", entry.SourcePaths)
	}
}

func TestReconcileManagedSourcePathsSkipsAmbiguousHashOwners(t *testing.T) {
	report := catalog.Report{Managed: []catalog.Entry{
		{Provider: "modrinth", ProjectID: "one", SHA512: "shared", SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/one.jar"}}},
		{Provider: "curseforge", ProjectID: "two", SHA512: "shared", SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/two.jar"}}},
	}}
	inv := inventory.Inventory{Mods: []inventory.ModFile{{
		Location: inventory.LocationServer,
		Path: "mods/moved.jar",
		SHA512: "shared",
	}}}

	if changed := reconcileManagedSourcePaths(inv, &report); changed != 0 {
		t.Fatalf("changed = %d, want ambiguous hash ownership left untouched", changed)
	}
}
