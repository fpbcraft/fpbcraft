package management

import (
	"testing"
	"time"

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


func TestBuildModsPrefersJarMetadataNameOverPersistedManagedName(t *testing.T) {
	inv := inventory.Inventory{Mods: []inventory.ModFile{{
		Location: inventory.LocationServer,
		Path:     "mods/ding.jar",
		Filename: "ding-1.5.0.jar",
		SHA512:   "ding-sha",
		Metadata: []inventory.ModMetadata{{Name: "Ding", Version: "1.5.0"}},
	}}}
	cat := catalog.Report{Managed: []catalog.Entry{{
		Provider:  "modrinth",
		ProjectID: "ding",
		Name:      "[1.21 NeoForge] v1.5.0",
		SHA512:    "ding-sha",
		SourcePaths: []catalog.Source{{
			Location: inventory.LocationServer,
			Path:     "mods/ding.jar",
		}},
	}}}

	mods := BuildMods(inv, cat)
	if len(mods) != 1 {
		t.Fatalf("mods = %d, want 1", len(mods))
	}
	if got := mods[0].Name; got != "Ding" {
		t.Fatalf("display name = %q, want JAR metadata name %q", got, "Ding")
	}
}

func TestBuildModsKeepsSiblingArtifactsDistinctAndShowsPreferredPlacement(t *testing.T) {
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		GeneratedAt: time.Now().UTC(),
		Mods: []inventory.ModFile{
			{Location: inventory.LocationServer, Path: "mods/recorder.jar", Filename: "recorder.jar", SHA512: "recorder"},
			{Location: inventory.LocationServer, Path: "mods/bluemap.jar", Filename: "bluemap.jar", SHA512: "bluemap"},
		},
	}
	cat := catalog.Report{
		Managed: []catalog.Entry{
			{
				Provider: "github", ProjectID: "fpbcraft/player-history-mc",
				Filename: "recorder.jar", SHA512: "recorder", Deployment: inventory.LocationServer,
				SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/recorder.jar"}},
			},
			{
				Provider: "github", ProjectID: "fpbcraft/player-history-mc",
				Filename: "bluemap.jar", SHA512: "bluemap", Deployment: inventory.LocationClient,
				SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/bluemap.jar"}},
			},
		},
	}
	catalog.EnsureManagedArtifactIDs(cat.Managed)
	mods := BuildMods(inv, cat)
	if len(mods) != 2 {
		t.Fatalf("mods = %d", len(mods))
	}
	if mods[0].ID == mods[1].ID {
		t.Fatalf("sibling artifacts share ID %q", mods[0].ID)
	}
	var bluemap Mod
	for _, mod := range mods {
		if mod.Filename == "bluemap.jar" {
			bluemap = mod
		}
	}
	if bluemap.Deployment != inventory.LocationServer || bluemap.PreferredDeployment != inventory.LocationClient {
		t.Fatalf("placement current=%q preferred=%q", bluemap.Deployment, bluemap.PreferredDeployment)
	}
}
