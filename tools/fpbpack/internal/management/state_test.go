package management

import (
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestBuildModsPreservesManagementIdentityWhenBytesDrift(t *testing.T) {
	inv := inventory.Inventory{Mods: []inventory.ModFile{
		{Location: inventory.LocationServer, Path: "mods/create.jar", Filename: "create.jar", SHA512: "changed", Metadata: []inventory.ModMetadata{{Name: "Create", Version: "6.0.10"}}},
		{Location: inventory.LocationServer, Path: "mods/custom.jar", Filename: "custom.jar", SHA512: "changed-custom"},
	}}
	cat := catalog.Report{
		Managed: []catalog.Entry{{
			Provider: "modrinth", ProjectID: "create", Name: "Create", SHA512: "accepted",
			SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/create.jar"}},
		}},
		Pinned: []catalog.PinnedArtifact{{
			SHA512: "accepted-custom", Filename: "custom.jar",
			Sources: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/custom.jar"}},
		}},
	}

	mods := BuildMods(inv, cat)
	if got := mods[0].Management; got != "managed" {
		t.Fatalf("managed artifact with drift classified as %q", got)
	}
	if got := mods[1].Management; got != "unmanaged" {
		t.Fatalf("pinned artifact with drift classified as %q", got)
	}
}

func TestBuildModsUsesDomainFields(t *testing.T) {
	inv := inventory.Inventory{Mods: []inventory.ModFile{{
		Location: inventory.LocationClient,
		Path:     "automodpack/host-modpack/main/mods/test.jar",
		Filename: "test.jar",
		SHA512:   "abc",
		Metadata: []inventory.ModMetadata{{Name: "Test Mod", Version: "1.2.3"}},
	}}}
	cat := catalog.Report{Managed: []catalog.Entry{{
		Provider: "modrinth", ProjectID: "project-id", Name: "Test Mod", SHA512: "abc", Side: "client",
	}}}

	mods := BuildMods(inv, cat)
	if len(mods) != 1 {
		t.Fatalf("len(mods) = %d, want 1", len(mods))
	}
	mod := mods[0]
	if mod.ID != "modrinth:project-id" || mod.InstalledVersion != "1.2.3" || mod.ProjectURL == "" {
		t.Fatalf("unexpected mod view: %#v", mod)
	}
}
