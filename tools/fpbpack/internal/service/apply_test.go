package service

import (
	"archive/zip"
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
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/doctor"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)


func TestFormatBlockingDiagnosticsIncludesActionableContext(t *testing.T) {
	report := doctor.Report{
		Summary: doctor.Summary{Blocking: 2},
		Findings: []doctor.Finding{
			{
				Code: "managed_artifact_moved",
				Level: doctor.LevelBlocking,
				Message: "managed artifact expected at mods/example.jar but matching bytes now exist elsewhere",
				Mod: "Example",
				Path: "mods/example.jar",
			},
			{
				Code: "external_artifact_added",
				Level: doctor.LevelBlocking,
				Message: "artifact exists in a managed deployment directory but is not present in the accepted catalog state",
				Mod: "Extra Mod",
				Path: "mods/extra.jar",
			},
		},
	}

	got := formatBlockingDiagnostics(report, 5)
	for _, want := range []string{
		"2 blocking diagnostic(s)",
		"[managed_artifact_moved]",
		"Example (mods/example.jar)",
		"managed artifact expected at mods/example.jar",
		"[external_artifact_added]",
		"Extra Mod (mods/extra.jar)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted diagnostics %q missing %q", got, want)
		}
	}
}

func TestFormatBlockingDiagnosticsTruncatesWithRemainingCount(t *testing.T) {
	report := doctor.Report{
		Summary: doctor.Summary{Blocking: 3},
		Findings: []doctor.Finding{
			{Code: "one", Level: doctor.LevelBlocking, Message: "first"},
			{Code: "two", Level: doctor.LevelBlocking, Message: "second"},
			{Code: "three", Level: doctor.LevelBlocking, Message: "third"},
		},
	}

	got := formatBlockingDiagnostics(report, 2)
	if !strings.Contains(got, "+1 more") {
		t.Fatalf("formatted diagnostics %q missing remaining count", got)
	}
	if strings.Contains(got, "[three]") {
		t.Fatalf("formatted diagnostics %q exceeded detail limit", got)
	}
}

func TestApplyAndRestoreMovesPreferredPlacementSafely(t *testing.T) {
	serverRoot := t.TempDir()
	stateDir := t.TempDir()
	oldPath := filepath.Join(serverRoot, "mods", "example-1.jar")
	oldSHA := writeServiceTestJar(t, oldPath, "example", "1.0.0")

	targetCache := filepath.Join(stateDir, "cache", "example-2.jar")
	targetSHA := writeServiceTestJar(t, targetCache, "example", "2.0.0")

	modrinth := newEmptyModrinthServer(t)
	crafty, _ := newTestCraftyServer(t, false)

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
			Provider: "modrinth",
			ProjectID: "example",
			VersionID: "old-version",
			Name: "Example",
			Filename: "example-1.jar",
			SHA512: oldSHA,
			Side: "both",
			Deployment: inventory.LocationClient,
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
			Minecraft: "1.21.1",
			Loader: "neoforge",
			ModrinthBaseURL: modrinth.URL,
			CraftyURL: crafty.URL,
			CraftyServerID: "server-1",
			CraftyToken: "test-token",
		},
		state: State{
			SchemaVersion: StateSchemaVersion,
			CreatedAt: now,
			UpdatedAt: now,
			Settings: RuntimeSettings{RetentionCount: DefaultRetentionCount},
			Catalog: cat,
		},
	}
	svc.snapshot = management.BuildSnapshot(inv, cat)

	verifiedAt := now
	plan := planning.Plan{
		SchemaVersion: planning.SchemaVersion,
		ID: "plan-0123456789abcdef",
		CreatedAt: now,
		Status: planning.StatusReady,
		InventoryGeneratedAt: inv.GeneratedAt,
		UpdatesGeneratedAt: now,
		Selected: []string{"modrinth:example"},
		Verified: true,
		VerifiedAt: &verifiedAt,
		RequiresServerStop: true,
		RequiresBackup: true,
		Prefetched: []planning.PrefetchedArtifact{{
			Filename: "example-2.jar",
			SHA512: targetSHA,
			CachePath: "cache/example-2.jar",
		}},
		Changes: []planning.Change{{
			CandidateKey: "modrinth:example",
			Name: "Example",
			Requested: true,
			Classification: updatecheck.ClassificationSafe,
			Installed: updatecheck.Release{ID: "old-version", Number: "1.0.0", SHA512: oldSHA},
			Target: updatecheck.Release{
				ID: "new-version",
				Number: "2.0.0",
				Filename: "example-2.jar",
				SHA512: targetSHA,
			},
			Artifact: planning.Artifact{
				Provider: "modrinth",
				ProjectID: "example",
				VersionID: "new-version",
				Filename: "example-2.jar",
				SHA512: targetSHA,
				Deployment: string(inventory.LocationClient),
			},
			Operations: []planning.FileOperation{{
				Action: "replace",
				CurrentPath: "mods/example-1.jar",
				TargetPath: filepath.ToSlash(filepath.Join(inventory.DefaultClientModsPath, "example-2.jar")),
				CurrentSHA512: oldSHA,
				TargetSHA512: targetSHA,
			}},
		}},
	}
	if err := writeJSONAtomic(filepath.Join(stateDir, "plans", plan.ID+".json"), plan); err != nil {
		t.Fatal(err)
	}

	result, err := svc.ApplyPlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "success" || result.BackupID == "" {
		t.Fatalf("unexpected apply result: %+v", result)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old server/common JAR still exists after placement move: %v", err)
	}
	appliedPath := filepath.Join(serverRoot, filepath.FromSlash(inventory.DefaultClientModsPath), "example-2.jar")
	if got, err := sha512File(appliedPath); err != nil || !strings.EqualFold(got, targetSHA) {
		t.Fatalf("applied target hash = %q err=%v", got, err)
	}

	appliedPlan, err := svc.Plan(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if appliedPlan.AppliedAt == nil {
		t.Fatal("applied plan did not record applied_at")
	}
	if len(svc.state.Catalog.Managed) != 1 ||
		svc.state.Catalog.Managed[0].Deployment != inventory.LocationClient ||
		len(svc.state.Catalog.Managed[0].SourcePaths) != 1 ||
		svc.state.Catalog.Managed[0].SourcePaths[0].Location != inventory.LocationClient {
		t.Fatalf("accepted catalog did not move to client placement: %+v", svc.state.Catalog.Managed)
	}

	restore, err := svc.RestoreBackup(context.Background(), result.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	if restore.Status != "success" {
		t.Fatalf("unexpected restore result: %+v", restore)
	}
	if _, err := os.Stat(appliedPath); !os.IsNotExist(err) {
		t.Fatalf("applied client JAR still exists after restore: %v", err)
	}
	if got, err := sha512File(oldPath); err != nil || !strings.EqualFold(got, oldSHA) {
		t.Fatalf("restored source hash = %q err=%v", got, err)
	}
	if svc.state.Catalog.Managed[0].Deployment != inventory.LocationClient {
		// The pre-apply accepted catalog intentionally remembers the preferred
		// placement. Restore puts the bytes back without discarding that choice.
		t.Fatalf("restore lost preferred placement: %+v", svc.state.Catalog.Managed[0])
	}
	if svc.state.Catalog.Managed[0].SourcePaths[0].Location != inventory.LocationServer {
		t.Fatalf("restore did not restore current source path: %+v", svc.state.Catalog.Managed[0].SourcePaths)
	}
}

func TestApplyAllowsUnrelatedManagedArtifactDrift(t *testing.T) {
	serverRoot := t.TempDir()
	stateDir := t.TempDir()
	firstPath := filepath.Join(serverRoot, "mods", "first.jar")
	firstSHA := writeServiceTestJar(t, firstPath, "first", "1.0.0")
	secondPath := filepath.Join(serverRoot, "mods", "second.jar")
	secondSHA := writeServiceTestJar(t, secondPath, "second", "1.0.0")
	targetCache := filepath.Join(stateDir, "cache", "first-2.jar")
	targetSHA := writeServiceTestJar(t, targetCache, "first", "2.0.0")

	crafty, _ := newTestCraftyServer(t, false)
	inv, err := inventory.Scan(inventory.ScanOptions{ServerRoot: serverRoot, ServerModsPath: "mods"})
	if err != nil {
		t.Fatal(err)
	}
	cat := catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		InventorySchema: inventory.SchemaVersion,
		Managed: []catalog.Entry{
			{
				Provider: "modrinth", ProjectID: "first", VersionID: "first-old",
				Name: "First", Filename: "first.jar", SHA512: firstSHA, Side: "both",
				Deployment: inventory.LocationServer,
				SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/first.jar"}},
			},
			{
				Provider: "modrinth", ProjectID: "second", VersionID: "second-old",
				Name: "Second", Filename: "second.jar", SHA512: secondSHA, Side: "both",
				Deployment: inventory.LocationServer,
				SourcePaths: []catalog.Source{{Location: inventory.LocationServer, Path: "mods/second.jar"}},
			},
		},
	}
	cat.RecalculateSummary()
	now := time.Now().UTC()
	svc := &Service{
		options: Options{
			ServerRoot: serverRoot,
			StateDir: stateDir,
			ServerModsPath: "mods",
			ClientModsPath: inventory.DefaultClientModsPath,
			CraftyURL: crafty.URL,
			CraftyServerID: "server-1",
			CraftyToken: "test-token",
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
		ID: "plan-fedcba9876543210",
		CreatedAt: now,
		Status: planning.StatusReady,
		Verified: true,
		RequiresServerStop: true,
		RequiresBackup: true,
		Prefetched: []planning.PrefetchedArtifact{{
			Filename: "first-2.jar", SHA512: targetSHA, CachePath: "cache/first-2.jar",
		}},
		Changes: []planning.Change{{
			CandidateKey: "modrinth:first",
			Name: "First",
			Requested: true,
			Classification: updatecheck.ClassificationSafe,
			Target: updatecheck.Release{ID: "first-new", Filename: "first-2.jar", SHA512: targetSHA},
			Artifact: planning.Artifact{
				Provider: "modrinth", ProjectID: "first", VersionID: "first-new",
				Filename: "first-2.jar", SHA512: targetSHA, Deployment: string(inventory.LocationServer),
			},
			Operations: []planning.FileOperation{{
				Action: "replace",
				CurrentPath: "mods/first.jar",
				TargetPath: "mods/first-2.jar",
				CurrentSHA512: firstSHA,
				TargetSHA512: targetSHA,
			}},
		}},
	}
	if err := writeJSONAtomic(filepath.Join(stateDir, "plans", plan.ID+".json"), plan); err != nil {
		t.Fatal(err)
	}

	svc.options.ModrinthBaseURL = newEmptyModrinthServer(t).URL
	_ = writeServiceTestJar(t, secondPath, "second", "1.0.1-manual")
	result, err := svc.ApplyPlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatalf("unrelated managed drift blocked a valid plan: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("apply status = %q", result.Status)
	}
	if got, hashErr := sha512File(filepath.Join(serverRoot, "mods", "first-2.jar")); hashErr != nil || !strings.EqualFold(got, targetSHA) {
		t.Fatalf("selected update was not installed: %q err=%v", got, hashErr)
	}
}

func newEmptyModrinthServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version_files" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	t.Cleanup(server.Close)
	return server
}

func writeServiceTestJar(t *testing.T, path, modID, version string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("META-INF/neoforge.mods.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(
		"[[mods]]\nmodId=\"" + modID + "\"\nversion=\"" + version + "\"\ndisplayName=\"" + modID + "\"\n",
	)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	sha, err := sha512File(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha
}


func TestApplyAndRestoreManagedRemoval(t *testing.T) {
	serverRoot := t.TempDir()
	stateDir := t.TempDir()
	modPath := filepath.Join(serverRoot, "mods", "remove-me.jar")
	modSHA := writeServiceTestJar(t, modPath, "remove_me", "1.0.0")
	crafty, _ := newTestCraftyServer(t, false)

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
			Provider: "modrinth",
			ProjectID: "remove-me",
			VersionID: "v1",
			Name: "Remove Me",
			Filename: "remove-me.jar",
			SHA512: modSHA,
			Side: "both",
			Deployment: inventory.LocationServer,
			SourcePaths: []catalog.Source{{
				Location: inventory.LocationServer,
				Path: "mods/remove-me.jar",
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
			CraftyURL: crafty.URL,
			CraftyServerID: "server-1",
			CraftyToken: "test-token",
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

	verifiedAt := now
	plan := planning.Plan{
		SchemaVersion: planning.SchemaVersion,
		ID: "plan-a1b2c3d4e5f60718",
		CreatedAt: now,
		Status: planning.StatusReady,
		Verified: true,
		VerifiedAt: &verifiedAt,
		RequiresServerStop: true,
		RequiresBackup: true,
		Selected: []string{"modrinth:remove-me"},
		Changes: []planning.Change{{
			CandidateKey: "modrinth:remove-me",
			Name: "Remove Me",
			Requested: true,
			Classification: updatecheck.ClassificationReview,
			Installed: updatecheck.Release{
				ID: "v1", Number: "1.0.0", Filename: "remove-me.jar", SHA512: modSHA,
			},
			Target: updatecheck.Release{
				ID: "v1", Number: "1.0.0", Filename: "remove-me.jar", SHA512: modSHA,
			},
			Artifact: planning.Artifact{
				Provider: "modrinth",
				ProjectID: "remove-me",
				VersionID: "v1",
				Filename: "remove-me.jar",
				SHA512: modSHA,
				Deployment: string(inventory.LocationServer),
			},
			Operations: []planning.FileOperation{{
				Action: "remove",
				CurrentPath: "mods/remove-me.jar",
				TargetPath: "mods/remove-me.jar",
				CurrentSHA512: modSHA,
			}},
		}},
	}
	if err := writeJSONAtomic(filepath.Join(stateDir, "plans", plan.ID+".json"), plan); err != nil {
		t.Fatal(err)
	}

	result, err := svc.ApplyPlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(modPath); !os.IsNotExist(err) {
		t.Fatalf("removed JAR still exists: %v", err)
	}
	if len(svc.state.Catalog.Managed) != 0 {
		t.Fatalf("removed mod remains accepted: %+v", svc.state.Catalog.Managed)
	}
	if result.BackupID == "" {
		t.Fatal("remove Apply did not create a restore point")
	}

	restore, err := svc.RestoreBackup(context.Background(), result.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	if restore.Status != "success" {
		t.Fatalf("restore result = %+v", restore)
	}
	if got, err := sha512File(modPath); err != nil || !strings.EqualFold(got, modSHA) {
		t.Fatalf("restored removed JAR hash=%q err=%v", got, err)
	}
	if len(svc.state.Catalog.Managed) != 1 ||
		svc.state.Catalog.Managed[0].ProjectID != "remove-me" {
		t.Fatalf("accepted catalog was not restored: %+v", svc.state.Catalog.Managed)
	}
}
