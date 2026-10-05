package catalog

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestResolveSourceRegistryVerifiesGitHubAssetBySHA512(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake packwiz helper uses a POSIX shell")
	}

	asset := []byte("exact release asset")
	sum := sha512.Sum512(asset)
	hash := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/owner/repo/releases/download/v1.0/custom.jar" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(asset)
	}))
	defer server.Close()

	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		ModrinthChecked: true,
		Mods: []inventory.ModFile{{
			Location: inventory.LocationServer,
			Path: "mods/custom.jar",
			Filename: "custom.jar",
			SHA1: "0123456789abcdef0123456789abcdef01234567",
			SHA512: hash,
			Metadata: []inventory.ModMetadata{{Name: "Custom GitHub Mod", Version: "1.0"}},
		}},
	}
	result, err := Build(inv)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "modpack")
	if err := Write(result, Options{OutputPath: output}); err != nil {
		t.Fatal(err)
	}

	registryPath := filepath.Join(t.TempDir(), "sources.json")
	writeRegistry(t, registryPath, SourceRegistry{
		SchemaVersion: SourceRegistrySchemaVersion,
		Sources: []SourceRule{{
			SHA512: hash,
			Type: "github_release",
			Repository: "owner/repo",
			Tag: "v1.0",
			Asset: "custom.jar",
		}},
	})

	packwiz := fakePackwiz(t)
	summary, err := ResolveSourceRegistry(context.Background(), inv, &result, SourceResolveOptions{
		RegistryPath: registryPath,
		OutputPath: output,
		PackwizPath: packwiz,
		GitHubBaseURL: server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.GitHubVerified != 1 || summary.Pinned != 0 || summary.Remaining != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(result.Report.Unresolved) != 0 || len(result.Report.Managed) != 1 {
		t.Fatalf("unexpected report after resolution: %+v", result.Report)
	}
	entry := result.Report.Managed[0]
	if entry.Provider != "github" || entry.Repository != "owner/repo" || entry.Tag != "v1.0" || entry.Asset != "custom.jar" {
		t.Fatalf("unexpected GitHub entry: %+v", entry)
	}
	metaPath := filepath.Join(output, "mods", metafileName(entry))
	meta, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(meta), server.URL+"/owner/repo/releases/download/v1.0/custom.jar") {
		t.Fatalf("GitHub metafile does not contain verified download URL:\n%s", meta)
	}
}

func TestResolveSourceRegistryRejectsGitHubHashMismatch(t *testing.T) {
	installed := sha512.Sum512([]byte("installed"))
	installedHash := hex.EncodeToString(installed[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("different remote asset"))
	}))
	defer server.Close()

	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		ModrinthChecked: true,
		Mods: []inventory.ModFile{{
			Location: inventory.LocationServer,
			Path: "mods/custom.jar",
			Filename: "custom.jar",
			SHA512: installedHash,
		}},
	}
	result, err := Build(inv)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "modpack")
	if err := Write(result, Options{OutputPath: output}); err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(t.TempDir(), "sources.json")
	writeRegistry(t, registryPath, SourceRegistry{
		SchemaVersion: SourceRegistrySchemaVersion,
		Sources: []SourceRule{{
			SHA512: installedHash,
			Type: "github_release",
			Repository: "owner/repo",
			Tag: "v1.0",
			Asset: "custom.jar",
		}},
	})

	_, err = ResolveSourceRegistry(context.Background(), inv, &result, SourceResolveOptions{
		RegistryPath: registryPath,
		OutputPath: output,
		PackwizPath: "unused",
		GitHubBaseURL: server.URL,
		HTTPClient: server.Client(),
	})
	if err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("expected hash mismatch, got %v", err)
	}
	if len(result.Report.Unresolved) != 1 || len(result.Report.Managed) != 0 {
		t.Fatalf("hash mismatch must leave artifact unresolved: %+v", result.Report)
	}
}

func TestResolveSourceRegistryAcceptsExplicitPinnedLocalArtifact(t *testing.T) {
	hash := strings.Repeat("a", 128)
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		ModrinthChecked: true,
		Mods: []inventory.ModFile{{
			Location: inventory.LocationServer,
			Path: "mods/custom.jar",
			Filename: "custom.jar",
			SHA512: hash,
		}},
	}
	result, err := Build(inv)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "modpack")
	if err := Write(result, Options{OutputPath: output}); err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(t.TempDir(), "sources.json")
	writeRegistry(t, registryPath, SourceRegistry{
		SchemaVersion: SourceRegistrySchemaVersion,
		Sources: []SourceRule{{
			SHA512: hash,
			Type: "pinned_local",
			Reason: "custom local build",
		}},
	})

	summary, err := ResolveSourceRegistry(context.Background(), inv, &result, SourceResolveOptions{
		RegistryPath: registryPath,
		OutputPath: output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Pinned != 1 || summary.Remaining != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(result.Report.Pinned) != 1 || result.Report.Pinned[0].Reason != "custom local build" {
		t.Fatalf("unexpected pinned report: %+v", result.Report.Pinned)
	}
	if len(result.Report.Unresolved) != 0 || result.Report.Summary.PinnedArtifacts != 1 {
		t.Fatalf("pinned artifact should be accounted for, got %+v", result.Report)
	}
}

func writeRegistry(t *testing.T, path string, registry SourceRegistry) {
	t.Helper()
	content, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakePackwiz(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "packwiz")
	script := "#!/bin/sh\nset -eu\nif [ \"$1\" = \"refresh\" ]; then exit 0; fi\necho \"unexpected args: $*\" >&2\nexit 2\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
