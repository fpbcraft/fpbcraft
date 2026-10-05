package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
)

func TestCreatePlacementPlanBuildsVerifiedMoveWithoutProviderUpdate(t *testing.T) {
	serverRoot := t.TempDir()
	stateDir := t.TempDir()
	currentPath := filepath.Join(serverRoot, "mods", "example.jar")
	sha := writeServiceTestJar(t, currentPath, "example", "1.0.0")

	inv, err := inventory.Scan(inventory.ScanOptions{
		ServerRoot: serverRoot,
		ServerModsPath: "mods",
		ClientModsPath: inventory.DefaultClientModsPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	cat := catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		InventorySchema: inventory.SchemaVersion,
		Managed: []catalog.Entry{{
			Provider: "github",
			ProjectID: "fpbcraft/example",
			VersionID: "v1.0.0",
			Tag: "v1.0.0",
			Repository: "fpbcraft/example",
			Asset: "example.jar",
			Name: "Example",
			Filename: "example.jar",
			SHA512: sha,
			Side: "both",
			Deployment: inventory.LocationClient,
			SourcePaths: []catalog.Source{{
				Location: inventory.LocationServer,
				Path: "mods/example.jar",
			}},
		}},
	}
	cat.RecalculateSummary()
	now := time.Now().UTC()
	svc := &Service{
		options: Options{
			ServerRoot: serverRoot,
			StateDir: stateDir,
			ServerModsPath: "mods",
			ClientModsPath: inventory.DefaultClientModsPath,
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

	plan, err := svc.CreatePlacementPlan(context.Background(), "mods/example.jar")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "ready" || !plan.Verified || plan.BackupID == "" {
		t.Fatalf("placement plan not ready: %+v", plan)
	}
	if len(plan.Changes) != 1 || len(plan.Changes[0].Operations) != 1 {
		t.Fatalf("unexpected placement plan changes: %+v", plan.Changes)
	}
	op := plan.Changes[0].Operations[0]
	wantTarget := filepath.ToSlash(filepath.Join(inventory.DefaultClientModsPath, "example.jar"))
	if op.CurrentPath != "mods/example.jar" || op.TargetPath != wantTarget {
		t.Fatalf("placement operation = %+v, target %q", op, wantTarget)
	}
	if !strings.EqualFold(op.CurrentSHA512, sha) || !strings.EqualFold(op.TargetSHA512, sha) {
		t.Fatalf("placement plan should preserve bytes: %+v", op)
	}
	if len(plan.Prefetched) != 1 || !strings.EqualFold(plan.Prefetched[0].SHA512, sha) {
		t.Fatalf("current bytes were not cached for deterministic move: %+v", plan.Prefetched)
	}
	cachePath := filepath.Join(stateDir, filepath.FromSlash(plan.Prefetched[0].CachePath))
	if got, err := sha512File(cachePath); err != nil || !strings.EqualFold(got, sha) {
		t.Fatalf("placement cache hash = %q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "backups", plan.BackupID, "manifest.json")); err != nil {
		t.Fatalf("restore point missing: %v", err)
	}
}
