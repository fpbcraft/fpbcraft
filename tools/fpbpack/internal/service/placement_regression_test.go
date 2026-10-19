package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func dualPlacementService(t *testing.T) (*Service, string, string) {
	t.Helper()
	root, state := t.TempDir(), t.TempDir()
	serverPath, clientPath := "mods/async-old.jar", inventory.DefaultClientModsPath+"/async-old.jar"
	sha := writeServiceTestJar(t, filepath.Join(root, serverPath), "asynclogger", "1.0.0")
	if _, _, err := copyFileWithSHA512(filepath.Join(root, serverPath), filepath.Join(root, clientPath)); err != nil {
		t.Fatal(err)
	}
	cat := catalog.Report{SchemaVersion: catalog.ReportSchemaVersion, InventorySchema: inventory.SchemaVersion, Managed: []catalog.Entry{{Provider: "modrinth", ProjectID: "async", VersionID: "v1", Filename: "async-old.jar", Name: "Async Logger", SHA512: sha, Deployment: inventory.LocationServer, SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: serverPath}, {Location: inventory.LocationClient, Group: "main", Path: clientPath}}}}}
	modrinth := newEmptyModrinthServer(t)
	crafty, _ := newTestCraftyServer(t, false)
	svc := &Service{options: Options{ServerRoot: root, StateDir: state, ServerModsPath: "mods", ClientModsPath: inventory.DefaultClientModsPath, ModrinthBaseURL: modrinth.URL, CraftyURL: crafty.URL, CraftyServerID: "server-1", CraftyToken: "test-token"}, state: State{SchemaVersion: StateSchemaVersion, CreatedAt: time.Now(), Catalog: cat, Settings: RuntimeSettings{RetentionCount: DefaultRetentionCount}}}
	inv, err := inventory.Scan(inventory.ScanOptions{ServerRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	svc.snapshot = management.BuildSnapshot(inv, cat)
	return svc, serverPath, clientPath
}

func TestMultiPlacementUpdateApplyAndRestore(t *testing.T) {
	svc, serverPath, clientPath := dualPlacementService(t)
	targetCache := filepath.Join(svc.options.StateDir, "target.jar")
	targetSHA := writeServiceTestJar(t, targetCache, "asynclogger", "2.0.0")
	candidate := updatecheck.Candidate{Key: "modrinth:async", Provider: "modrinth", ProjectID: "async", Name: "Async Logger", Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationSafe, Target: &updatecheck.Release{ID: "v2", Filename: "async-new.jar", URL: "https://example.test/new.jar", SHA512: targetSHA}}
	plan, err := planning.Build([]string{candidate.Key}, updatecheck.Report{Candidates: []updatecheck.Candidate{candidate}}, svc.snapshot, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	plan.Verified = true
	plan.Prefetched = []planning.PrefetchedArtifact{{SHA512: targetSHA, CachePath: "target.jar"}}
	svc.validateProposedDependencies(&plan)
	if plan.Status != planning.StatusReady || len(plan.Changes[0].Operations) != 2 {
		t.Fatalf("not ready: %+v", plan)
	}
	if err := writeJSONAtomic(filepath.Join(svc.options.StateDir, "plans", plan.ID+".json"), plan); err != nil {
		t.Fatal(err)
	}
	applied, err := svc.ApplyPlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{serverPath, clientPath} {
		if _, err := os.Stat(filepath.Join(svc.options.ServerRoot, old)); !os.IsNotExist(err) {
			t.Fatalf("old copy survives: %s %v", old, err)
		}
		target := filepath.Join(svc.options.ServerRoot, filepath.Dir(old), "async-new.jar")
		if hash, err := sha512File(target); err != nil || hash != targetSHA {
			t.Fatalf("new copy missing: %s %v", target, err)
		}
	}
	if len(svc.state.Catalog.Managed) != 1 || len(svc.state.Catalog.Managed[0].SourcePaths) != 2 {
		t.Fatalf("catalog lost placements: %+v", svc.state.Catalog.Managed)
	}
	if _, err := svc.RestoreBackup(context.Background(), applied.BackupID); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{serverPath, clientPath} {
		if hash, err := sha512File(filepath.Join(svc.options.ServerRoot, path)); err != nil || hash != svc.state.Catalog.Managed[0].SHA512 {
			t.Fatalf("restore lost %s: %v", path, err)
		}
	}
}

func TestDuplicateCopyRemovalApplyAndRestore(t *testing.T) {
	svc, serverPath, clientPath := dualPlacementService(t)
	plan, err := svc.CreateCatalogPlan(context.Background(), CatalogPlanRequest{Action: "remove", Path: clientPath})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != planning.StatusReady || len(plan.Changes[0].Operations) != 1 {
		t.Fatalf("not ready: %+v", plan)
	}
	applied, err := svc.ApplyPlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(svc.options.ServerRoot, clientPath)); !os.IsNotExist(err) {
		t.Fatalf("copy not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.options.ServerRoot, serverPath)); err != nil {
		t.Fatal("server copy removed", err)
	}
	if len(svc.state.Catalog.Managed) != 1 || len(svc.state.Catalog.Managed[0].SourcePaths) != 1 || svc.state.Catalog.Managed[0].SourcePaths[0].Path != serverPath {
		t.Fatalf("remaining catalog entry lost: %+v", svc.state.Catalog.Managed)
	}
	if _, err := svc.RestoreBackup(context.Background(), applied.BackupID); err != nil {
		t.Fatal(err)
	}
	if len(svc.state.Catalog.Managed[0].SourcePaths) != 2 {
		t.Fatal("restore lost catalog placement")
	}
	if _, err := os.Stat(filepath.Join(svc.options.ServerRoot, clientPath)); err != nil {
		t.Fatal("restore lost removed copy", err)
	}
}

func TestDependencyReviewRejectsChangedUnselectedJar(t *testing.T) {
	svc, _, _ := dualPlacementService(t)
	plan := planning.Plan{Status: planning.StatusReady}
	svc.validateProposedDependencies(&plan)
	if plan.Status != planning.StatusReady {
		t.Fatalf("unexpected blockers: %+v", plan.Blockers)
	}
	writeServiceTestJar(t, filepath.Join(svc.options.ServerRoot, "mods/other.jar"), "other", "1.0")
	if err := svc.validateDependencySnapshot(plan); err == nil {
		t.Fatal("adding an unselected JAR must invalidate dependency review")
	}
}

func TestPendingRemovalsPreserveBothSelectedLocations(t *testing.T) {
	svc, serverPath, clientPath := dualPlacementService(t)
	for _, path := range []string{serverPath, clientPath} {
		if _, err := svc.StagePendingCatalogChange(context.Background(), CatalogPlanRequest{Action: "remove", Path: path}); err != nil {
			t.Fatal(err)
		}
	}
	if len(svc.state.PendingChanges.Changes) != 2 {
		t.Fatalf("one file selection overwrote another: %+v", svc.state.PendingChanges)
	}
	plan, err := svc.ReviewPendingChanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != planning.StatusReady || len(plan.Changes) != 1 || len(plan.Changes[0].Operations) != 2 {
		t.Fatalf("both selected paths must be reviewed: %+v", plan)
	}
}

func TestUpdateBlocksMissingIntendedCopy(t *testing.T) {
	svc, _, clientPath := dualPlacementService(t)
	if err := os.Remove(filepath.Join(svc.options.ServerRoot, clientPath)); err != nil {
		t.Fatal(err)
	}
	inv, err := inventory.Scan(inventory.ScanOptions{ServerRoot: svc.options.ServerRoot})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := management.BuildSnapshot(inv, svc.state.Catalog)
	candidate := updatecheck.Candidate{Key: "modrinth:async", Provider: "modrinth", ProjectID: "async", Classification: updatecheck.ClassificationSafe, Target: &updatecheck.Release{ID: "v2", Filename: "new.jar", URL: "https://example.test/new.jar", SHA512: "new"}}
	plan, err := planning.Build([]string{candidate.Key}, updatecheck.Report{Candidates: []updatecheck.Candidate{candidate}}, snapshot, time.Now())
	if err != nil || plan.Status != planning.StatusBlocked {
		t.Fatalf("missing placement must not be silently dropped: %v %+v", err, plan)
	}
}

func TestArtifactMigrationPreservesRulesAndInvalidatesReview(t *testing.T) {
	state := State{Catalog: catalog.Report{Managed: []catalog.Entry{{Provider: "modrinth", ProjectID: "async", ArtifactID: "primary", Aliases: []string{"modrinth:async#retired"}}}}, UpdateRules: map[string]UpdateRule{"modrinth:async#retired": {PinVersion: "v1"}}, PendingChanges: PendingChanges{ReviewedPlanID: "plan-old", Changes: []PendingChange{{ID: "modrinth:async#retired", CandidateKey: "modrinth:async#retired", Action: "update"}}}}
	if !normalizeArtifactReferences(&state) {
		t.Fatal("expected alias migration")
	}
	if state.UpdateRules["modrinth:async#primary"].PinVersion != "v1" || state.PendingChanges.ReviewedPlanID != "" || state.PendingChanges.Changes[0].CandidateKey != "modrinth:async#primary" {
		t.Fatalf("lost rules or selection: %+v", state)
	}
	if normalizeArtifactReferences(&state) {
		t.Fatal("migration must be idempotent")
	}
}

func TestMixedCopyRemovalAndUpdatePreservesCatalog(t *testing.T) {
	svc, serverPath, clientPath := dualPlacementService(t)
	plan := planning.Plan{Changes: []planning.Change{{CandidateKey: "modrinth:async", Name: "Async", Target: updatecheck.Release{ID: "v2"}, Artifact: planning.Artifact{Provider: "modrinth", ProjectID: "async", Filename: "new.jar", SHA512: "new", Deployment: "server"}, Operations: []planning.FileOperation{{Action: "remove", CurrentPath: clientPath, TargetPath: clientPath}, {Action: "replace", CurrentPath: serverPath, TargetPath: "mods/new.jar"}}}}}
	next, err := catalogAfterPlan(svc.state.Catalog, plan)
	if err != nil || len(next.Managed) != 1 || next.Managed[0].SHA512 != "new" || len(next.Managed[0].SourcePaths) != 1 || next.Managed[0].SourcePaths[0].Path != "mods/new.jar" {
		t.Fatalf("mixed plan lost release: %v %+v", err, next.Managed)
	}
}
