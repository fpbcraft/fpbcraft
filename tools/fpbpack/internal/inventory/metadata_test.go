package inventory

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestReadNeoForgeMetadata(t *testing.T) {
	path := writeTestJar(t, map[string]string{
		"META-INF/neoforge.mods.toml": `modLoader="javafml"
[[mods]]
modId="example"
version="${file.jarVersion}"
displayName="Example Mod"
`,
		"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\r\nImplementation-Version: 1.2.3\r\n\r\n",
	})

	metadata, err := ReadMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 1 {
		t.Fatalf("expected 1 metadata entry, got %d", len(metadata))
	}
	got := metadata[0]
	if got.Loader != "neoforge" || got.ModID != "example" || got.Name != "Example Mod" || got.Version != "1.2.3" {
		t.Fatalf("unexpected metadata: %+v", got)
	}
}

func TestReadNeoForgeMetadataIgnoresDependencyTables(t *testing.T) {
	path := writeTestJar(t, map[string]string{
		"META-INF/neoforge.mods.toml": `[[mods]]
modId="abridged"
version="2.0.2"
displayName="Abridged"

[[dependencies.abridged]]
modId="lithostitched"
type="required"
versionRange="[1.0,)"
`,
	})

	metadata, err := ReadMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 1 {
		t.Fatalf("expected 1 metadata entry, got %d", len(metadata))
	}
	if metadata[0].ModID != "abridged" || metadata[0].Name != "Abridged" || metadata[0].Version != "2.0.2" {
		t.Fatalf("dependency table overwrote mod metadata: %+v", metadata[0])
	}
}

func TestReadMultipleMods(t *testing.T) {
	path := writeTestJar(t, map[string]string{
		"META-INF/mods.toml": `[[mods]]
modId = "first"
version = "1.0"
[[mods]]
modId = "second"
version = "2.0"
`,
	})

	metadata, err := ReadMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 2 || metadata[0].ModID != "first" || metadata[1].ModID != "second" {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
}

func TestReadFabricMetadataFallback(t *testing.T) {
	path := writeTestJar(t, map[string]string{
		"fabric.mod.json": `{"id":"fabric-example","name":"Fabric Example","version":"4.5.6"}`,
	})

	metadata, err := ReadMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 1 || metadata[0].Loader != "fabric" || metadata[0].ModID != "fabric-example" {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
}

func writeTestJar(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.jar")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
