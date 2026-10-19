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

func TestReadDeclaredDependencyConstraints(t *testing.T) {
	path := writeTestJar(t, map[string]string{"META-INF/neoforge.mods.toml": `[[mods]]
modId='parent'
version='1.0'
[[dependencies.parent]]
modId='library'
type='required'
versionRange='[1.0,2.0)'
side='CLIENT'
[[dependencies.parent]]
modId='conflict'
type='incompatible'
versionRange='[2,)'
`})
	metadata, err := ReadMetadata(path)
	if err != nil || len(metadata) != 1 || len(metadata[0].Dependencies) != 2 {
		t.Fatalf("%v %+v", err, metadata)
	}
	dep := metadata[0].Dependencies[0]
	if dep.ModID != "library" || dep.Side != "client" || dep.Versions[0] != "[1.0,2.0)" {
		t.Fatalf("incorrect requirement: %+v", dep)
	}
}
func TestReadFabricDependencyAlternativesAndAliases(t *testing.T) {
	path := writeTestJar(t, map[string]string{"fabric.mod.json": `{"id":"parent","version":"1.0","provides":["alias"],"depends":{"library":["^1.0","^2.0"]},"breaks":{"bad":"*"}}`})
	metadata, err := ReadMetadata(path)
	if err != nil || len(metadata) != 1 || len(metadata[0].Dependencies) != 2 || len(metadata[0].Provides) != 1 {
		t.Fatalf("%v %+v", err, metadata)
	}
	if len(metadata[0].Dependencies[0].Versions) != 2 {
		t.Fatal("lost alternatives")
	}
}

func TestReadDeclaredEmbeddedMod(t *testing.T) {
	library := writeTestJar(t, map[string]string{"META-INF/neoforge.mods.toml": "[[mods]]\nmodId='library'\nversion='1.2.3'\n"})
	content, err := os.ReadFile(library)
	if err != nil {
		t.Fatal(err)
	}
	parent := writeTestJar(t, map[string]string{
		"META-INF/neoforge.mods.toml":   "[[mods]]\nmodId='parent'\nversion='1'\n[[dependencies.parent]]\nmodId='library'\ntype='required'\nversionRange='[1,2)'\n",
		"META-INF/jarjar/metadata.json": `{"jars":[{"path":"META-INF/jarjar/library.jar"}]}`,
		"META-INF/jarjar/library.jar":   string(content),
	})
	metadata, err := ReadMetadata(parent)
	if err != nil || len(metadata) != 2 || !metadata[1].Embedded || metadata[1].ModID != "library" {
		t.Fatalf("embedded library not discovered: %v %+v", err, metadata)
	}
}
