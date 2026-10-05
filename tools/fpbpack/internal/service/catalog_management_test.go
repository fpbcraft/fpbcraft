package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
)

func TestCreateCatalogInstallPlanFromExactModrinthVersion(t *testing.T) {
	serverRoot := t.TempDir()
	stateDir := t.TempDir()
	artifactDir := t.TempDir()
	artifactPath := filepath.Join(artifactDir, "catalog-install.jar")
	targetSHA := writeServiceTestJar(t, artifactPath, "catalog_install", "1.2.3")
	artifactBytes, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/version/v1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "v1",
				"project_id": "project-1",
				"name": "Catalog Install 1.2.3",
				"version_number": "1.2.3",
				"version_type": "release",
				"status": "listed",
				"date_published": "2026-10-05T12:00:00Z",
				"game_versions": []string{"1.21.1"},
				"loaders": []string{"neoforge"},
				"environment": "client_and_server",
				"dependencies": []any{},
				"files": []map[string]any{{
					"hashes": map[string]any{"sha512": targetSHA},
					"url": server.URL + "/artifact.jar",
					"filename": "catalog-install.jar",
					"primary": true,
				}},
			})
		case "/projects":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": "project-1",
				"title": "Catalog Install",
				"slug": "catalog-install",
				"icon_url": "",
			}})
		case "/artifact.jar":
			w.Header().Set("Content-Type", "application/java-archive")
			_, _ = w.Write(artifactBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	now := time.Now().UTC()
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		GeneratedAt: now,
		ServerModsPath: "mods",
		ClientModsPath: inventory.DefaultClientModsPath,
		ModrinthChecked: true,
		Mods: []inventory.ModFile{},
	}
	cat := catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		InventorySchema: inventory.SchemaVersion,
	}
	cat.RecalculateSummary()
	svc := &Service{
		options: Options{
			ServerRoot: serverRoot,
			StateDir: stateDir,
			ServerModsPath: "mods",
			ClientModsPath: inventory.DefaultClientModsPath,
			Minecraft: "1.21.1",
			Loader: "neoforge",
			ModrinthBaseURL: server.URL,
		},
		state: State{
			SchemaVersion: StateSchemaVersion,
			CreatedAt: now,
			UpdatedAt: now,
			Settings: RuntimeSettings{RetentionCount: DefaultRetentionCount},
			Catalog: cat,
		},
		snapshot: management.BuildSnapshot(inv, cat),
	}

	plan, err := svc.CreateCatalogPlan(context.Background(), CatalogPlanRequest{
		Action: "install",
		Provider: "modrinth",
		ProjectID: "project-1",
		VersionID: "v1",
		Placement: inventory.LocationServer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "ready" || !plan.Verified || plan.BackupID == "" {
		t.Fatalf("catalog install plan not ready/protected: %+v", plan)
	}
	if len(plan.Changes) != 1 || len(plan.Changes[0].Operations) != 1 {
		t.Fatalf("catalog install plan changes = %+v", plan.Changes)
	}
	change := plan.Changes[0]
	if change.Operations[0].Action != "add" ||
		change.Operations[0].TargetPath != "mods/catalog-install.jar" {
		t.Fatalf("catalog install operation = %+v", change.Operations[0])
	}
	if change.Artifact.VersionID != "v1" ||
		change.Target.Number != "1.2.3" ||
		!strings.EqualFold(change.Artifact.SHA512, targetSHA) {
		t.Fatalf("exact provider target was not preserved: %+v", change)
	}
	if len(plan.Prefetched) != 1 {
		t.Fatalf("prefetched artifacts = %+v", plan.Prefetched)
	}
	cachePath := filepath.Join(stateDir, filepath.FromSlash(plan.Prefetched[0].CachePath))
	if got, err := sha512File(cachePath); err != nil || !strings.EqualFold(got, targetSHA) {
		t.Fatalf("prefetched target hash=%q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "backups", plan.BackupID, "manifest.json")); err != nil {
		t.Fatalf("install restore point missing: %v", err)
	}
}
