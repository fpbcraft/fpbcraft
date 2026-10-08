package inventory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanPreservesServerAndClientLocations(t *testing.T) {
	root := t.TempDir()
	serverDir := filepath.Join(root, "mods")
	clientDir := filepath.Join(root, "automodpack", "host-modpack", "main", "mods")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		t.Fatal(err)
	}

	serverJar := writeTestJar(t, map[string]string{"fabric.mod.json": `{"id":"servermod","version":"1"}`})
	clientJar := writeTestJar(t, map[string]string{"fabric.mod.json": `{"id":"clientmod","version":"2"}`})
	copyFile(t, serverJar, filepath.Join(serverDir, "server.jar"))
	copyFile(t, clientJar, filepath.Join(clientDir, "client.jar"))
	if err := os.WriteFile(filepath.Join(serverDir, "ignore.txt"), []byte("not a jar"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(ScanOptions{ServerRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Total != 2 || result.Summary.Server != 1 || result.Summary.Client != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if result.Mods[0].Location != LocationClient || result.Mods[1].Location != LocationServer {
		t.Fatalf("unexpected locations/order: %+v", result.Mods)
	}
}

func TestScanContinuesPastUnreadableJarMetadata(t *testing.T) {
	root := t.TempDir()
	serverDir := filepath.Join(root, "mods")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "broken.jar"), []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	validJar := writeTestJar(t, map[string]string{"fabric.mod.json": `{"id":"valid","version":"1"}`})
	copyFile(t, validJar, filepath.Join(serverDir, "valid.jar"))

	result, err := Scan(ScanOptions{ServerRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Total != 2 || result.Summary.MetadataUnreadable != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
}

func copyFile(t *testing.T, source, destination string) {
	t.Helper()
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, content, 0o644); err != nil {
		t.Fatal(err)
	}
}


func TestScanDiscoversAllAutoModpackGroups(t *testing.T) {
	root := t.TempDir()
	for _, group := range []string{"main", "performance", "visual"} {
		dir := filepath.Join(root, "automodpack", "host-modpack", group, "mods")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		jar := writeTestJar(t, map[string]string{"fabric.mod.json": `{"id":"` + group + `","version":"1"}`})
		copyFile(t, jar, filepath.Join(dir, group+".jar"))
	}
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(ScanOptions{ServerRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Client != 3 || result.Summary.ClientGroups != 3 {
		t.Fatalf("unexpected client summary: %+v", result.Summary)
	}
	for _, group := range []string{"main", "performance", "visual"} {
		expectedPath := filepath.ToSlash(filepath.Join(DefaultAutoModpackHostPath, group, "mods"))
		if result.ClientGroupModsPaths[group] != expectedPath {
			t.Fatalf("group %s path = %q, want %q", group, result.ClientGroupModsPaths[group], expectedPath)
		}
		found := false
		for _, mod := range result.Mods {
			if mod.Group == group && mod.Location == LocationClient {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("group %s was not assigned to its discovered JAR: %+v", group, result.Mods)
		}
	}
}

func TestScanReportsEveryInspectedJar(t *testing.T) {
	root := t.TempDir()
	serverDir := filepath.Join(root, "mods")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := writeTestJar(t, map[string]string{"fabric.mod.json": `{"id":"testmod","version":"1"}`})
	copyFile(t, jar, filepath.Join(serverDir, "example.jar"))
	var paths []string
	inv, err := Scan(ScanOptions{
		ServerRoot: root,
		OnFile: func(mod ModFile) { paths = append(paths, mod.Path) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Mods) != 1 || len(paths) != 1 || paths[0] != "mods/example.jar" {
		t.Fatalf("unexpected per-file reports: %v (inventory: %d)", paths, len(inv.Mods))
	}
}

func TestScanReusesCachedInspectionAndInvalidatesOnChange(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "mods")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := writeTestJar(t, map[string]string{"fabric.mod.json": `{"id":"cached","version":"1"}`})
	path := filepath.Join(dir, "cached.jar")
	copyFile(t, jar, path)

	initial, err := Scan(ScanOptions{ServerRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.Mods) != 1 || initial.Mods[0].ModifiedUnixNano == 0 {
		t.Fatalf("missing inspection metadata: %+v", initial.Mods)
	}

	reused, err := Scan(ScanOptions{ServerRoot: root, Previous: &initial})
	if err != nil {
		t.Fatal(err)
	}
	if !reused.Mods[0].Cached || reused.Mods[0].SHA512 != initial.Mods[0].SHA512 {
		t.Fatalf("expected cache hit for unchanged JAR: %+v", reused.Mods[0])
	}

	updated := writeTestJar(t, map[string]string{"fabric.mod.json": `{"id":"cached","version":"2"}`})
	copyFile(t, updated, path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Force mtime to change even on filesystems with coarse timestamps.
	if err := os.Chtimes(path, info.ModTime().Add(2*time.Second), info.ModTime().Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	changed, err := Scan(ScanOptions{ServerRoot: root, Previous: &initial})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Mods[0].Cached || changed.Mods[0].SHA512 == initial.Mods[0].SHA512 {
		t.Fatalf("modified artifact was incorrectly reused: %+v", changed.Mods[0])
	}
}
