package inventory

import (
	"os"
	"path/filepath"
	"testing"
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
