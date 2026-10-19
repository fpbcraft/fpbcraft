package catalog

import (
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"testing"
)

func TestCoalesceExactReleasePreservesPathsAndRetiredKeys(t *testing.T) {
	r := Report{Managed: []Entry{
		{Provider: "modrinth", ProjectID: "async", ArtifactID: "client", SHA512: "HASH", SourcePaths: []Source{{Location: inventory.LocationClient, Group: "main", Path: "automodpack/host-modpack/main/mods/async.jar"}}},
		{Provider: "modrinth", ProjectID: "async", ArtifactID: "server", SHA512: "hash", SourcePaths: []Source{{Location: inventory.LocationServer, Path: "mods/async.jar"}}},
	}}
	if !r.CoalesceManaged() {
		t.Fatal("expected merge")
	}
	if len(r.Managed) != 1 || len(r.Managed[0].SourcePaths) != 2 || !MatchesKey(r.Managed[0], "modrinth:async#server") {
		t.Fatalf("lost identity or paths: %+v", r.Managed)
	}
	if r.CoalesceManaged() {
		t.Fatal("normalization must be idempotent")
	}
}

func TestCoalesceKeepsIndependentAssetsAndVersions(t *testing.T) {
	r := Report{Managed: []Entry{
		{Provider: "github", ProjectID: "repo", SHA512: "recorder"},
		{Provider: "github", ProjectID: "repo", SHA512: "addon"},
		{Provider: "modrinth", ProjectID: "almanac", SHA512: "old"},
		{Provider: "modrinth", ProjectID: "almanac", SHA512: "new"},
	}}
	if r.CoalesceManaged() || len(r.Managed) != 4 {
		t.Fatal("distinct byte identities must survive")
	}
	if got := UniqueProjects(r.Managed, "modrinth")["almanac"]; got.SHA512 != "" || got.VersionID != "" {
		t.Fatalf("ambiguous project selected a release: %+v", got)
	}
}
