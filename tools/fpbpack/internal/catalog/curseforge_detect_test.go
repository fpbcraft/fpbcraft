package catalog

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestResolveCurseForgeWithPackwizUsesIsolatedCopies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake packwiz helper uses a POSIX shell")
	}

	serverRoot := t.TempDir()
	serverMods := filepath.Join(serverRoot, "mods")
	if err := os.MkdirAll(serverMods, 0o755); err != nil {
		t.Fatal(err)
	}
	liveJar := filepath.Join(serverMods, "cupboard-1.21.1-4.2.jar")
	if err := os.WriteFile(liveJar, []byte("fake jar"), 0o644); err != nil {
		t.Fatal(err)
	}

	inv := inventory.Inventory{
		SchemaVersion:   inventory.SchemaVersion,
		ServerRoot:      serverRoot,
		ModrinthChecked: true,
		Mods: []inventory.ModFile{{
			Location: inventory.LocationServer,
			Path:     "mods/cupboard-1.21.1-4.2.jar",
			Filename: "cupboard-1.21.1-4.2.jar",
			SHA1:     "0123456789abcdef0123456789abcdef01234567",
			SHA512:   "cf-sha512",
		}},
	}
	result, err := Build(inv)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "modpack")
	if err := Write(result, Options{OutputPath: output, Minecraft: "1.21.1"}); err != nil {
		t.Fatal(err)
	}

	fakePackwiz := filepath.Join(t.TempDir(), "packwiz")
	script := strings.Join([]string{
		"#!/bin/sh",
		"set -eu",
		"if [ \"$1\" = \"curseforge\" ] && [ \"$2\" = \"detect\" ]; then",
		"  rm -f \"mods/cupboard-1.21.1-4.2.jar\"",
		"  cat > \"mods/cupboard.pw.toml\" <<'EOF'",
		"name = \"Cupboard\"",
		"filename = \"cupboard-1.21.1-4.2.jar\"",
		"side = \"both\"",
		"",
		"[download]",
		"hash-format = \"sha1\"",
		"hash = \"0123456789abcdef0123456789abcdef01234567\"",
		"mode = \"metadata:curseforge\"",
		"",
		"[update.curseforge]",
		"file-id = 8889050",
		"project-id = 326652",
		"EOF",
		"  echo \"Detection complete!\"",
		"  exit 0",
		"fi",
		"if [ \"$1\" = \"refresh\" ]; then exit 0; fi",
		"echo \"unexpected fake packwiz args: $*\" >&2",
		"exit 2",
		"",
	}, "\n")
	if err := os.WriteFile(fakePackwiz, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	summary, err := ResolveCurseForgeWithPackwiz(inv, &result, output, fakePackwiz)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Detected != 1 || summary.Unmatched != 0 {
		t.Fatalf("unexpected detection summary: %+v", summary)
	}
	if _, err := os.Stat(liveJar); err != nil {
		t.Fatalf("live server jar was modified: %v", err)
	}
	if len(result.Report.Unresolved) != 0 {
		t.Fatalf("expected unresolved list to be empty, got %+v", result.Report.Unresolved)
	}
	if len(result.Report.Managed) != 1 {
		t.Fatalf("expected one managed entry, got %+v", result.Report.Managed)
	}
	entry := result.Report.Managed[0]
	if entry.Provider != "curseforge" || entry.ProjectID != "326652" || entry.FileID != 8889050 {
		t.Fatalf("unexpected managed entry: %+v", entry)
	}
	report, err := os.ReadFile(filepath.Join(output, "migration-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), "\"provider\": \"curseforge\"") {
		t.Fatalf("migration report was not rewritten with CurseForge entry:\n%s", report)
	}
}

func TestResolveCurseForgeWithPackwizRemovesUnmatchedTemporaryJars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake packwiz helper uses a POSIX shell")
	}
	serverRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(serverRoot, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	liveJar := filepath.Join(serverRoot, "mods", "custom.jar")
	if err := os.WriteFile(liveJar, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion, ServerRoot: serverRoot, ModrinthChecked: true,
		Mods: []inventory.ModFile{{Location: inventory.LocationServer, Path: "mods/custom.jar", Filename: "custom.jar", SHA512: "custom-sha"}},
	}
	result, err := Build(inv)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "modpack")
	if err := Write(result, Options{OutputPath: output}); err != nil {
		t.Fatal(err)
	}
	fakePackwiz := filepath.Join(t.TempDir(), "packwiz")
	script := strings.Join([]string{
		"#!/bin/sh",
		"set -eu",
		"if [ \"$1\" = \"curseforge\" ]; then echo \"Detection complete!\"; exit 0; fi",
		"if [ \"$1\" = \"refresh\" ]; then exit 0; fi",
		"exit 2",
		"",
	}, "\n")
	if err := os.WriteFile(fakePackwiz, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	summary, err := ResolveCurseForgeWithPackwiz(inv, &result, output, fakePackwiz)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Detected != 0 || summary.Unmatched != 1 {
		t.Fatalf("unexpected detection summary: %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(output, "mods", "custom.jar")); !os.IsNotExist(err) {
		t.Fatalf("temporary unmatched jar should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(liveJar); err != nil {
		t.Fatalf("live jar was modified: %v", err)
	}
	if len(result.Report.Unresolved) != 1 {
		t.Fatalf("unmatched artifact should remain unresolved: %+v", result.Report.Unresolved)
	}
}
