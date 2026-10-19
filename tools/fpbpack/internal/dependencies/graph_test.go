package dependencies

import (
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"testing"
)

func testFile(path, id, version string, location inventory.Location, group string, deps ...inventory.Requirement) inventory.ModFile {
	return inventory.ModFile{Path: path, SHA512: path, Location: location, Group: group, Metadata: []inventory.ModMetadata{{ModID: id, Version: version, Loader: "neoforge", Dependencies: deps}}}
}
func hasCode(findings []Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
func TestUnselectedDependentConstrainsLibrary(t *testing.T) {
	files := []inventory.ModFile{testFile("mods/custom.jar", "custom", "1", inventory.LocationServer, "", inventory.Requirement{ModID: "library", Type: "required", Versions: []string{"[1.0,2.0)"}}), testFile("mods/library.jar", "library", "2.0", inventory.LocationServer, "")}
	if !hasCode(Analyze(files), "required_mod_version") {
		t.Fatal("unmanaged/unselected dependent constraint ignored")
	}
	files[1].Metadata[0].Version = "1.5"
	if got := Analyze(files); len(got) != 0 {
		t.Fatalf("compatible library rejected: %+v", got)
	}
	if !hasCode(Analyze(files[:1]), "required_mod_missing") {
		t.Fatal("removal was not checked")
	}
}
func TestClientDependencyVisibility(t *testing.T) {
	required := inventory.Requirement{ModID: "library", Type: "required", Versions: []string{"[1,)"}}
	parent := testFile("client/main.jar", "parent", "1", inventory.LocationClient, "main", required)
	library := testFile("client/optional/library.jar", "library", "1", inventory.LocationClient, "optional")
	if !hasCode(Analyze([]inventory.ModFile{parent, library}), "required_mod_missing") {
		t.Fatal("optional group incorrectly satisfies main")
	}
	parent.Group = "optional"
	library.Group = "main"
	if got := Analyze([]inventory.ModFile{parent, library}); len(got) != 0 {
		t.Fatalf("main must satisfy optional group: %+v", got)
	}
	library.Location = inventory.LocationServer
	if !hasCode(Analyze([]inventory.ModFile{parent, library}), "dependency_delivery_unverified") {
		t.Fatal("server/common is not verified client delivery")
	}
}
func TestDuplicatesUseRuntimeIDsNotProviderProject(t *testing.T) {
	a := testFile("mods/recorder.jar", "recorder", "1", inventory.LocationServer, "")
	b := testFile("mods/addon.jar", "addon", "1", inventory.LocationServer, "")
	if len(Analyze([]inventory.ModFile{a, b})) != 0 {
		t.Fatal("independent assets must be supported")
	}
	b.Metadata[0].ModID = "recorder"
	if !hasCode(Analyze([]inventory.ModFile{a, b}), "runtime_mod_duplicate") {
		t.Fatal("duplicate runtime ID missed")
	}
	b.Location = inventory.LocationClient
	b.Metadata[0].Version = "2"
	if !hasCode(Analyze([]inventory.ModFile{a, b}), "runtime_version_mismatch") {
		t.Fatal("cross-location version mismatch missed")
	}
}
func TestVersionConstraints(t *testing.T) {
	for _, tc := range []struct {
		version, expression, loader string
		match, unknown              bool
	}{
		{"1.5", "[1.0,2.0)", "neoforge", true, false},
		{"2.0", "[1.0,2.0)", "neoforge", false, false},
		{"2.0", "[2.0]", "neoforge", true, false},
		{"3.0", "(,1.0],[3.0,)", "neoforge", true, false},
		{"1.5.0", ">=1.0 <2.0", "fabric", true, false},
		{"1.5.0", "^1.0", "fabric", true, false},
		{"custom-build", "[1,)", "neoforge", false, true},
		{"v1.5", "[1,2)", "neoforge", false, true},
		{"2.0-beta", "[1,)", "neoforge", false, true},
	} {
		ok, err := Matches(tc.version, []string{tc.expression}, tc.loader)
		if ok != tc.match || (err != nil) != tc.unknown {
			t.Errorf("%+v: match=%v error=%v", tc, ok, err)
		}
	}
}

func TestOptionalDependencyChecksPresentVersion(t *testing.T) {
	parent := testFile("mods/parent.jar", "parent", "1", inventory.LocationServer, "", inventory.Requirement{ModID: "library", Type: "optional", Versions: []string{"[1,2)"}})
	if got := Analyze([]inventory.ModFile{parent}); len(got) != 0 {
		t.Fatalf("missing optional dependency blocks: %+v", got)
	}
	library := testFile("mods/library.jar", "library", "2.1", inventory.LocationServer, "")
	if !hasCode(Analyze([]inventory.ModFile{parent, library}), "required_mod_version") {
		t.Fatal("incompatible optional dependency was accepted")
	}
}

func TestMavenExactVersionIsNotWildcard(t *testing.T) {
	ok, err := Matches("1.2", []string{"[1]"}, "neoforge")
	if err != nil || ok {
		t.Fatalf("[1] incorrectly matched 1.2: %v %v", ok, err)
	}
}

func TestEmbeddedDependencyDoesNotInstallSecondJar(t *testing.T) {
	parent := testFile("mods/parent.jar", "parent", "1", inventory.LocationServer, "", inventory.Requirement{ModID: "library", Type: "required", Versions: []string{"[1,2)"}})
	parent.Metadata = append(parent.Metadata, inventory.ModMetadata{ModID: "library", Version: "1.2", Loader: "neoforge", Embedded: true})
	if got := Analyze([]inventory.ModFile{parent}); len(got) != 0 {
		t.Fatalf("embedded library incorrectly missing: %+v", got)
	}
}
