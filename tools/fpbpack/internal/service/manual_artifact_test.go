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
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func TestAcceptManualArtifactVerifiesChecksumAndReadiesPlan(t *testing.T) {
	serverRoot := t.TempDir()
	stateDir := t.TempDir()
	currentPath := filepath.Join(serverRoot, "mods", "example-1.jar")
	currentSHA := writeServiceTestJar(t, currentPath, "example", "1.0.0")

	downloadDir := t.TempDir()
	downloadPath := filepath.Join(downloadDir, "example-2.jar")
	targetSHA := writeServiceTestJar(t, downloadPath, "example", "2.0.0")
	targetHashes, err := artifactFileHashes(downloadPath)
	if err != nil {
		t.Fatal(err)
	}

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
			Provider: "curseforge",
			ProjectID: "123",
			FileID: 1,
			Name: "Example",
			Filename: "example-1.jar",
			SHA1: inv.Mods[0].SHA1,
			SHA512: currentSHA,
			Side: "both",
			Deployment: inventory.LocationServer,
			SourcePaths: []catalog.Source{{
				Location: inventory.LocationServer,
				Path: "mods/example-1.jar",
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

	plan := planning.Plan{
		SchemaVersion: planning.SchemaVersion,
		ID: "plan-1111111111111111",
		CreatedAt: now,
		Status: planning.StatusBlocked,
		Selected: []string{"curseforge:123"},
		RequiresServerStop: true,
		RequiresBackup: true,
		Blockers: []planning.Finding{{
			Code: "manual_download_required",
			CandidateKey: "curseforge:123",
			Message: "manual download required",
		}},
		Changes: []planning.Change{{
			CandidateKey: "curseforge:123",
			Name: "Example",
			Requested: true,
			Classification: updatecheck.ClassificationReview,
			Installed: updatecheck.Release{ID: "1", Number: "1.0.0", SHA512: currentSHA},
			Target: updatecheck.Release{
				ID: "2",
				Number: "2.0.0",
				Filename: "example-2.jar",
				SHA1: targetHashes.SHA1,
				ManualDownload: true,
			},
			Artifact: planning.Artifact{
				Provider: "curseforge",
				ProjectID: "123",
				VersionID: "2",
				Filename: "example-2.jar",
				SHA1: targetHashes.SHA1,
				Deployment: string(inventory.LocationServer),
				ManualDownload: true,
				ManualURL: "https://example.invalid/files/2",
			},
			Operations: []planning.FileOperation{{
				Action: "replace",
				CurrentPath: "mods/example-1.jar",
				TargetPath: "mods/example-2.jar",
				CurrentSHA512: currentSHA,
			}},
		}},
	}
	if err := writeJSONAtomic(filepath.Join(stateDir, "plans", plan.ID+".json"), plan); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(downloadPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	updated, err := svc.AcceptManualArtifact(
		context.Background(),
		plan.ID,
		"curseforge:123",
		file,
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != planning.StatusReady || !updated.Verified || updated.BackupID == "" {
		t.Fatalf("manual artifact did not ready the plan: %+v", updated)
	}
	change := updated.Changes[0]
	if !change.Artifact.ManualProvided {
		t.Fatal("manual artifact was not marked provided")
	}
	if !strings.EqualFold(change.Artifact.SHA512, targetSHA) ||
		!strings.EqualFold(change.Operations[0].TargetSHA512, targetSHA) {
		t.Fatalf("normalized SHA-512 missing: %+v", change)
	}
	if len(updated.Blockers) != 0 {
		t.Fatalf("manual blocker was not cleared: %+v", updated.Blockers)
	}
	if len(updated.Prefetched) != 1 {
		t.Fatalf("prefetched = %d, want 1", len(updated.Prefetched))
	}
	cachePath := filepath.Join(stateDir, filepath.FromSlash(updated.Prefetched[0].CachePath))
	if got, err := sha512File(cachePath); err != nil || !strings.EqualFold(got, targetSHA) {
		t.Fatalf("cached manual artifact hash = %q err=%v", got, err)
	}
	if got, err := sha512File(currentPath); err != nil || !strings.EqualFold(got, currentSHA) {
		t.Fatalf("manual verification mutated live JAR: %q err=%v", got, err)
	}
}

func TestAcceptManualArtifactRejectsWrongJar(t *testing.T) {
	stateDir := t.TempDir()
	now := time.Now().UTC()
	expected := t.TempDir() + "/expected.jar"
	_ = writeServiceTestJar(t, expected, "expected", "1")
	hashes, err := artifactFileHashes(expected)
	if err != nil {
		t.Fatal(err)
	}
	wrong := t.TempDir() + "/wrong.jar"
	_ = writeServiceTestJar(t, wrong, "wrong", "1")

	svc := &Service{
		options: Options{StateDir: stateDir},
		state: State{
			SchemaVersion: StateSchemaVersion,
			CreatedAt: now,
			UpdatedAt: now,
			Settings: RuntimeSettings{RetentionCount: DefaultRetentionCount},
			Catalog: catalog.Report{SchemaVersion: catalog.ReportSchemaVersion},
		},
	}
	plan := planning.Plan{
		SchemaVersion: planning.SchemaVersion,
		ID: "plan-2222222222222222",
		CreatedAt: now,
		Status: planning.StatusBlocked,
		Blockers: []planning.Finding{{
			Code: "manual_download_required",
			CandidateKey: "curseforge:123",
		}},
		Changes: []planning.Change{{
			CandidateKey: "curseforge:123",
			Name: "Expected",
			Artifact: planning.Artifact{
				ManualDownload: true,
				Filename: "expected.jar",
				SHA1: hashes.SHA1,
			},
		}},
	}
	if err := writeJSONAtomic(filepath.Join(stateDir, "plans", plan.ID+".json"), plan); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(wrong)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := svc.AcceptManualArtifact(context.Background(), plan.ID, "curseforge:123", file); err == nil {
		t.Fatal("wrong manual JAR was accepted")
	}
	persisted, err := svc.Plan(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Changes[0].Artifact.ManualProvided {
		t.Fatal("failed manual upload mutated persisted plan")
	}
}
