package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestBuildDeduplicatesExactArtifactsAndDetectsVersionConflicts(t *testing.T) {
	inv := inventory.Inventory{SchemaVersion: 1, ModrinthChecked: true, Mods: []inventory.ModFile{
		{Location: inventory.LocationClient, Path: "client/a.jar", Filename: "a.jar", SHA512: "same", Modrinth: mr("p1", "v1", "1.0", "client_and_server")},
		{Location: inventory.LocationServer, Path: "mods/a.jar", Filename: "a.jar", SHA512: "same", Modrinth: mr("p1", "v1", "1.0", "client_and_server")},
		{Location: inventory.LocationServer, Path: "mods/b1.jar", Filename: "b1.jar", SHA512: "b1", Modrinth: mr("p2", "v1", "1.0", "client_and_server")},
		{Location: inventory.LocationServer, Path: "mods/b2.jar", Filename: "b2.jar", SHA512: "b2", Modrinth: mr("p2", "v2", "2.0", "client_and_server")},
		{Location: inventory.LocationServer, Path: "mods/custom.jar", Filename: "custom.jar", SHA512: "custom"},
	}}
	result, err := Build(inv)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || result.Entries[0].ProjectID != "p1" || result.Entries[0].Deployment != inventory.LocationServer {
		t.Fatalf("unexpected entries: %+v", result.Entries)
	}
	if result.Report.Summary.DuplicateArtifacts != 1 || result.Report.Summary.ConflictProjects != 1 || result.Report.Summary.Unresolved != 1 {
		t.Fatalf("unexpected report summary: %+v", result.Report.Summary)
	}
}

func TestWritePackwizCatalog(t *testing.T) {
	inv := inventory.Inventory{SchemaVersion: 1, ModrinthChecked: true, Mods: []inventory.ModFile{
		{Location: inventory.LocationClient, Path: "client/a.jar", Filename: "a.jar", SHA512: "abc", Modrinth: mr("project", "version", "1.0", "client_only")},
	}}
	result, err := Build(inv)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "modpack")
	if err := Write(result, Options{OutputPath: out, Minecraft: "1.21.1", NeoForge: "21.1.200"}); err != nil {
		t.Fatal(err)
	}
	metafile, err := os.ReadFile(filepath.Join(out, "mods", "project.pw.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(metafile)
	if !strings.Contains(text, `side = "client"`) || !strings.Contains(text, `mod-id = "project"`) || !strings.Contains(text, `version = "version"`) {
		t.Fatalf("unexpected metafile:\n%s", text)
	}
	pack, err := os.ReadFile(filepath.Join(out, "pack.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pack), `neoforge = "21.1.200"`) {
		t.Fatalf("missing neoforge version: %s", pack)
	}
	report, err := os.ReadFile(filepath.Join(out, "migration-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(report, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Summary.GeneratedProjects != 1 {
		t.Fatalf("unexpected report: %+v", decoded)
	}
}


func TestWriteCurseForgeMetafile(t *testing.T) {
	inv := inventory.Inventory{SchemaVersion: inventory.SchemaVersion, ModrinthChecked: true, CurseForgeChecked: true, Mods: []inventory.ModFile{
		{
			Location: inventory.LocationServer,
			Path: "mods/cupboard.jar",
			Filename: "cupboard-1.21.1-4.2.jar",
			SHA1: "0123456789abcdef0123456789abcdef01234567",
			SHA512: "cf-sha512",
			CurseForgeFingerprint: 197930586,
			CurseForge: &inventory.CurseForgeMatch{
				ProjectID: 326652,
				FileID: 8889050,
				DisplayName: "Cupboard 1.21.1-4.2",
				Filename: "cupboard-1.21.1-4.2.jar",
			},
		},
	}}
	result, err := Build(inv)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || result.Entries[0].Provider != "curseforge" || result.Entries[0].ProjectID != "326652" {
		t.Fatalf("unexpected entries: %+v", result.Entries)
	}
	out := filepath.Join(t.TempDir(), "modpack")
	if err := Write(result, Options{OutputPath: out, Minecraft: "1.21.1"}); err != nil {
		t.Fatal(err)
	}
	metafile, err := os.ReadFile(filepath.Join(out, "mods", "curseforge-326652.pw.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(metafile)
	for _, expected := range []string{
		`hash-format = "sha1"`,
		`hash = "0123456789abcdef0123456789abcdef01234567"`,
		`mode = "metadata:curseforge"`,
		`file-id = 8889050`,
		`project-id = 326652`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in metafile:\n%s", expected, text)
		}
	}
}


func TestPlacementMismatchOnlyFlagsStrictSideViolations(t *testing.T) {
	tests := []struct {
		name        string
		deployment  inventory.Location
		environment string
		want        bool
	}{
		{"client-only on server", inventory.LocationServer, "client_only", true},
		{"server-only on client", inventory.LocationClient, "server_only", true},
		{"client optional on server", inventory.LocationServer, "client_only_server_optional", false},
		{"server optional on client", inventory.LocationClient, "server_only_client_optional", false},
		{"both on server", inventory.LocationServer, "client_and_server", false},
		{"both on client", inventory.LocationClient, "client_and_server", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := placementMismatch(tt.deployment, tt.environment); got != tt.want {
				t.Fatalf("placementMismatch(%q, %q) = %v, want %v", tt.deployment, tt.environment, got, tt.want)
			}
		})
	}
}


func mr(project, version, number, environment string) *inventory.ModrinthMatch {
	return &inventory.ModrinthMatch{ProjectID: project, VersionID: version, VersionNumber: number, VersionName: project + " " + number, Filename: project + ".jar", URL: "https://cdn.example/" + project + ".jar", Environment: environment}
}
