package planning

import (
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func TestBuildCreatesDeterministicReadyPlan(t *testing.T) {
	invTime := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	updateTime := invTime.Add(time.Minute)
	report := updatecheck.Report{
		GeneratedAt: updateTime,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:create", Provider: "modrinth", ProjectID: "create",
			Name: "Create", Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationSafe,
			Installed: updatecheck.Release{ID: "old", Number: "1.0"},
			Target: &updatecheck.Release{
				ID: "new", Number: "1.1", Filename: "create-1.1.jar",
				URL: "https://cdn.example/create.jar", SHA512: "target",
			},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: invTime},
		Mods: []management.Mod{{
			Provider: "modrinth", ProjectID: "create", Path: "mods/create-1.0.jar",
			SHA512: "current",
		}},
	}

	first, err := Build([]string{"modrinth:create"}, report, snapshot, invTime.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build([]string{"modrinth:create"}, report, snapshot, invTime.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("plan IDs differ for same inputs: %s != %s", first.ID, second.ID)
	}
	if first.Status != StatusReady || len(first.Blockers) != 0 {
		t.Fatalf("unexpected plan readiness: %+v", first)
	}
	if got := first.Changes[0].Operations[0].TargetPath; got != "mods/create-1.1.jar" {
		t.Fatalf("target path = %q", got)
	}
}

func TestBuildBlocksUnresolvedDependencyClosure(t *testing.T) {
	report := updatecheck.Report{
		GeneratedAt: time.Now().UTC(),
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:test", Provider: "modrinth", ProjectID: "test", Name: "Test",
			Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationReview,
			Target: &updatecheck.Release{
				ID: "new", Filename: "test.jar", URL: "https://cdn.example/test.jar", SHA512: "target",
			},
			Dependencies: []updatecheck.Dependency{{
				Type: "required", Action: "add", Provider: "modrinth", ProjectID: "dep",
			}},
		}},
	}
	snapshot := management.Snapshot{
		Mods: []management.Mod{{Provider: "modrinth", ProjectID: "test", Path: "mods/test-old.jar"}},
	}
	plan, err := Build([]string{"modrinth:test"}, report, snapshot, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusBlocked {
		t.Fatalf("status = %s, want blocked", plan.Status)
	}
	found := false
	for _, blocker := range plan.Blockers {
		if blocker.Code == "dependency_target_not_resolved" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected dependency blocker: %+v", plan.Blockers)
	}
}

func TestBuildAddsResolvedDependencyChanges(t *testing.T) {
	invTime := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: invTime.Add(time.Minute),
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:main",
			Provider: "modrinth",
			ProjectID: "main",
			Name: "Main",
			Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Installed: updatecheck.Release{ID: "main-old", Number: "1.0"},
			Target: &updatecheck.Release{
				ID: "main-new", Number: "1.1", Filename: "main-new.jar",
				URL: "https://cdn.example/main.jar", SHA512: "main-target",
			},
			Dependencies: []updatecheck.Dependency{{
				Provider: "modrinth",
				ProjectID: "dep",
				Name: "Dependency",
				Type: "required",
				Action: "add",
				Deployment: inventory.LocationClient,
				TargetVersion: "dep-v1",
				Target: &updatecheck.Release{
					ID: "dep-v1", Number: "2.0", Filename: "dep.jar",
					URL: "https://cdn.example/dep.jar", SHA512: "dep-target",
				},
			}},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{
			GeneratedAt: invTime,
			ServerModsPath: "mods",
			ClientModsPath: "automodpack/host-modpack/main/mods",
		},
		Mods: []management.Mod{{
			Provider: "modrinth", ProjectID: "main", Path: "mods/main-old.jar",
			SHA512: "main-current",
		}},
	}

	plan, err := Build([]string{"modrinth:main"}, report, snapshot, invTime.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusReady {
		t.Fatalf("status = %s, blockers=%+v", plan.Status, plan.Blockers)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("changes = %d, want 2: %+v", len(plan.Changes), plan.Changes)
	}
	var dependency Change
	for _, change := range plan.Changes {
		if change.CandidateKey == "modrinth:dep" {
			dependency = change
		}
	}
	if !dependency.DependencyDriven || dependency.Requested {
		t.Fatalf("dependency flags are wrong: %+v", dependency)
	}
	if len(dependency.Operations) != 1 || dependency.Operations[0].Action != "add" {
		t.Fatalf("unexpected dependency operation: %+v", dependency.Operations)
	}
	if got := dependency.Operations[0].TargetPath; got != "automodpack/host-modpack/main/mods/dep.jar" {
		t.Fatalf("target path = %q", got)
	}
	if !plan.RequiresBackup {
		t.Fatal("main replacement should require a restore point")
	}
}

func TestBuildBlocksConflictingDependencyTargets(t *testing.T) {
	invTime := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	makeCandidate := func(project, target, dependencyTarget string) updatecheck.Candidate {
		return updatecheck.Candidate{
			Key: "modrinth:" + project, Provider: "modrinth", ProjectID: project,
			Name: project, Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Target: &updatecheck.Release{
				ID: target, Filename: project + ".jar", URL: "https://cdn.example/" + project,
				SHA512: target + "-hash",
			},
			Dependencies: []updatecheck.Dependency{{
				Provider: "modrinth", ProjectID: "shared", Type: "required", Action: "add",
				Deployment: inventory.LocationServer, TargetVersion: dependencyTarget,
				Target: &updatecheck.Release{
					ID: dependencyTarget, Filename: "shared.jar",
					URL: "https://cdn.example/shared-" + dependencyTarget,
					SHA512: dependencyTarget + "-hash",
				},
			}},
		}
	}
	report := updatecheck.Report{
		GeneratedAt: invTime.Add(time.Minute),
		Candidates: []updatecheck.Candidate{
			makeCandidate("one", "one-new", "shared-a"),
			makeCandidate("two", "two-new", "shared-b"),
		},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: invTime, ServerModsPath: "mods"},
		Mods: []management.Mod{
			{Provider: "modrinth", ProjectID: "one", Path: "mods/one-old.jar"},
			{Provider: "modrinth", ProjectID: "two", Path: "mods/two-old.jar"},
		},
	}
	plan, err := Build([]string{"modrinth:one", "modrinth:two"}, report, snapshot, invTime.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusBlocked {
		t.Fatalf("expected conflicting targets to block: %+v", plan)
	}
	found := false
	for _, blocker := range plan.Blockers {
		if blocker.Code == "dependency_target_conflict" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing dependency target conflict: %+v", plan.Blockers)
	}
}

func TestBuildBlocksTargetPathOccupiedByUnmanagedArtifact(t *testing.T) {
	invTime := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: invTime.Add(time.Minute),
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:main",
			Provider: "modrinth",
			ProjectID: "main",
			Name: "Main",
			Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Installed: updatecheck.Release{ID: "main-old"},
			Target: &updatecheck.Release{
				ID: "main-new", Filename: "main-new.jar",
				URL: "https://cdn.example/main.jar", SHA512: "main-target",
			},
			Dependencies: []updatecheck.Dependency{{
				Provider: "modrinth", ProjectID: "dep", Name: "Dependency",
				Type: "required", Action: "add", Deployment: inventory.LocationServer,
				Target: &updatecheck.Release{
					ID: "dep-v1", Filename: "occupied.jar",
					URL: "https://cdn.example/dep.jar", SHA512: "dep-target",
				},
			}},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: invTime, ServerModsPath: "mods"},
		Mods: []management.Mod{
			{Provider: "modrinth", ProjectID: "main", Name: "Main", Path: "mods/main-old.jar"},
			{Name: "Pinned Custom", Filename: "occupied.jar", Management: "unmanaged", Path: "mods/occupied.jar"},
		},
	}
	plan, err := Build([]string{"modrinth:main"}, report, snapshot, invTime.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusBlocked {
		t.Fatalf("expected occupied target to block: %+v", plan)
	}
	found := false
	for _, blocker := range plan.Blockers {
		if blocker.Code == "target_path_occupied" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing target_path_occupied blocker: %+v", plan.Blockers)
	}
}


func TestBuildPlansSiblingArtifactsFromSameProviderProject(t *testing.T) {
	now := time.Now().UTC()
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{
			{
				Key: "github:fpbcraft/player-history-mc#recorder",
				Provider: "github", ProjectID: "fpbcraft/player-history-mc", Name: "Recorder",
				Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationReview,
				Installed: updatecheck.Release{ID: "v1", Number: "v1"},
				Target: &updatecheck.Release{
					ID: "v2", Number: "v2", Filename: "recorder-v2.jar",
					URL: "https://example.invalid/recorder.jar", SHA512: "recorder-target",
				},
			},
			{
				Key: "github:fpbcraft/player-history-mc#bluemap",
				Provider: "github", ProjectID: "fpbcraft/player-history-mc", Name: "BlueMap addon",
				Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationReview,
				Installed: updatecheck.Release{ID: "v1", Number: "v1"},
				Target: &updatecheck.Release{
					ID: "v2", Number: "v2", Filename: "bluemap-v2.jar",
					URL: "https://example.invalid/bluemap.jar", SHA512: "bluemap-target",
				},
			},
		},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: now},
		Mods: []management.Mod{
			{
				ID: "github:fpbcraft/player-history-mc#recorder",
				Provider: "github", ProjectID: "fpbcraft/player-history-mc",
				Path: "mods/recorder-v1.jar", SHA512: "recorder-current",
			},
			{
				ID: "github:fpbcraft/player-history-mc#bluemap",
				Provider: "github", ProjectID: "fpbcraft/player-history-mc",
				Path: "mods/bluemap-v1.jar", SHA512: "bluemap-current",
			},
		},
	}
	plan, err := Build(
		[]string{
			"github:fpbcraft/player-history-mc#recorder",
			"github:fpbcraft/player-history-mc#bluemap",
		},
		report,
		snapshot,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusReady || len(plan.Changes) != 2 {
		t.Fatalf("unexpected multi-artifact plan: status=%s changes=%d blockers=%+v", plan.Status, len(plan.Changes), plan.Blockers)
	}
	if plan.Changes[0].CandidateKey == plan.Changes[1].CandidateKey {
		t.Fatalf("sibling artifacts were coalesced: %+v", plan.Changes)
	}
}


func TestBuildTreatsExactManagedDependencyAtTargetAsAlreadySatisfied(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now.Add(time.Minute),
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:main",
			Provider: "modrinth",
			ProjectID: "main",
			Name: "Main",
			Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Installed: updatecheck.Release{ID: "main-old"},
			Target: &updatecheck.Release{
				ID: "main-new",
				Filename: "main-new.jar",
				URL: "https://cdn.example/main.jar",
				SHA512: "main-target",
			},
			Dependencies: []updatecheck.Dependency{{
				Provider: "modrinth",
				ProjectID: "architectury-modrinth",
				Name: "Architectury API",
				Type: "required",
				Action: "add",
				Deployment: inventory.LocationServer,
				Target: &updatecheck.Release{
					ID: "architectury-v13",
					Filename: "architectury-13.0.11-neoforge.jar",
					URL: "https://cdn.example/architectury.jar",
					SHA512: "architectury-exact",
				},
			}},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: now, ServerModsPath: "mods"},
		Mods: []management.Mod{
			{
				ID: "modrinth:main",
				Provider: "modrinth",
				ProjectID: "main",
				Name: "Main",
				Management: "managed",
				Deployment: inventory.LocationServer,
				Path: "mods/main-old.jar",
				SHA512: "main-old-hash",
			},
			{
				ID: "curseforge:architectury",
				Provider: "curseforge",
				ProjectID: "architectury",
				Name: "[NeoForge 1.21] v13.0.11",
				Filename: "architectury-13.0.11-neoforge.jar",
				Management: "managed",
				Deployment: inventory.LocationServer,
				Path: "mods/architectury-13.0.11-neoforge.jar",
				SHA512: "architectury-exact",
			},
		},
	}

	plan, err := Build([]string{"modrinth:main"}, report, snapshot, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusReady {
		t.Fatalf("exact existing managed dependency should not block: %+v", plan.Blockers)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].CandidateKey != "modrinth:main" {
		t.Fatalf("satisfied dependency should not create a filesystem change: %+v", plan.Changes)
	}
	for _, blocker := range plan.Blockers {
		if blocker.Code == "target_path_occupied" {
			t.Fatalf("exact managed dependency was incorrectly treated as occupied: %+v", blocker)
		}
	}
	foundWarning := false
	for _, warning := range plan.Warnings {
		if warning.Code == "dependency_already_satisfied" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("expected dependency satisfaction note: %+v", plan.Warnings)
	}
}

func TestBuildStillBlocksDifferentManagedArtifactAtDependencyTarget(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now.Add(time.Minute),
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:main",
			Provider: "modrinth",
			ProjectID: "main",
			Name: "Main",
			Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Target: &updatecheck.Release{
				ID: "main-new", Filename: "main-new.jar",
				URL: "https://cdn.example/main.jar", SHA512: "main-target",
			},
			Dependencies: []updatecheck.Dependency{{
				Provider: "modrinth", ProjectID: "dep", Name: "Dependency",
				Type: "required", Action: "add", Deployment: inventory.LocationServer,
				Target: &updatecheck.Release{
					ID: "dep-v1", Filename: "dep.jar",
					URL: "https://cdn.example/dep.jar", SHA512: "required-hash",
				},
			}},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: now, ServerModsPath: "mods"},
		Mods: []management.Mod{
			{
				ID: "modrinth:main", Provider: "modrinth", ProjectID: "main",
				Name: "Main", Management: "managed", Deployment: inventory.LocationServer,
				Path: "mods/main-old.jar", SHA512: "main-old",
			},
			{
				ID: "curseforge:other", Provider: "curseforge", ProjectID: "other",
				Name: "Different bytes", Filename: "dep.jar", Management: "managed",
				Deployment: inventory.LocationServer, Path: "mods/dep.jar", SHA512: "different-hash",
			},
		},
	}

	plan, err := Build([]string{"modrinth:main"}, report, snapshot, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusBlocked {
		t.Fatalf("different occupied bytes must still block: %+v", plan)
	}
	found := false
	for _, blocker := range plan.Blockers {
		if blocker.Code == "target_path_occupied" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing occupied-target blocker: %+v", plan.Blockers)
	}
}


func TestBuildCatalogInstallCreatesAddOperation(t *testing.T) {
	now := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:new-mod",
			Provider: "modrinth",
			ProjectID: "new-mod",
			Name: "New Mod",
			Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Intent: "install",
			Target: &updatecheck.Release{
				ID: "version-1",
				Number: "1.0.0",
				Filename: "new-mod-1.0.0.jar",
				URL: "https://cdn.example/new-mod.jar",
				SHA512: "target-sha",
			},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{
			GeneratedAt: now,
			ServerModsPath: "mods",
			ClientModsPath: inventory.DefaultClientModsPath,
		},
		Mods: []management.Mod{},
	}

	plan, err := Build([]string{"modrinth:new-mod"}, report, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusReady {
		t.Fatalf("install plan blocked: %+v", plan.Blockers)
	}
	if len(plan.Changes) != 1 || len(plan.Changes[0].Operations) != 1 {
		t.Fatalf("unexpected install plan: %+v", plan.Changes)
	}
	op := plan.Changes[0].Operations[0]
	if op.Action != "add" || op.CurrentPath != "" || op.TargetPath != "mods/new-mod-1.0.0.jar" {
		t.Fatalf("install operation = %+v", op)
	}
	if plan.RequiresBackup {
		t.Fatal("pure install should not require backing up an existing JAR")
	}
	if !plan.RequiresServerStop {
		t.Fatal("install still requires the server stopped")
	}
}

func TestBuildCatalogRemoveBlocksRequiredDependency(t *testing.T) {
	now := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:library",
			Provider: "modrinth",
			ProjectID: "library",
			Name: "Library",
			Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Intent: "remove",
			RequiredBy: []string{"Dependent Mod"},
			Installed: updatecheck.Release{
				ID: "lib-v1",
				Number: "1.0.0",
				Filename: "library.jar",
				SHA512: "library-sha",
			},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: now, ServerModsPath: "mods"},
		Mods: []management.Mod{{
			ID: "modrinth:library",
			Provider: "modrinth",
			ProjectID: "library",
			Name: "Library",
			Filename: "library.jar",
			Management: "managed",
			Deployment: inventory.LocationServer,
			Path: "mods/library.jar",
			SHA512: "library-sha",
		}},
	}

	plan, err := Build([]string{"modrinth:library"}, report, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusBlocked {
		t.Fatalf("required dependency removal should block: %+v", plan)
	}
	found := false
	for _, blocker := range plan.Blockers {
		if blocker.Code == "required_dependency_remove" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing required dependency blocker: %+v", plan.Blockers)
	}
}

func TestBuildCatalogRemoveCreatesRestorableRemoveOperation(t *testing.T) {
	now := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "curseforge:123",
			Provider: "curseforge",
			ProjectID: "123",
			Name: "Optional Mod",
			Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Intent: "remove",
			Installed: updatecheck.Release{
				ID: "456",
				Number: "1.0.0",
				Filename: "optional.jar",
				SHA512: "optional-sha",
			},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: now, ServerModsPath: "mods"},
		Mods: []management.Mod{{
			ID: "curseforge:123",
			Provider: "curseforge",
			ProjectID: "123",
			Name: "Optional Mod",
			Filename: "optional.jar",
			Management: "managed",
			Deployment: inventory.LocationServer,
			Path: "mods/optional.jar",
			SHA512: "optional-sha",
		}},
	}

	plan, err := Build([]string{"curseforge:123"}, report, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusReady {
		t.Fatalf("remove plan blocked: %+v", plan.Blockers)
	}
	op := plan.Changes[0].Operations[0]
	if op.Action != "remove" || op.CurrentPath != "mods/optional.jar" || op.TargetPath != "mods/optional.jar" {
		t.Fatalf("remove operation = %+v", op)
	}
	if !plan.RequiresBackup || !plan.RequiresServerStop {
		t.Fatalf("remove protections backup=%v stop=%v", plan.RequiresBackup, plan.RequiresServerStop)
	}
}


func TestExplicitCatalogInstallDoesNotCoalesceOccupiedManagedTarget(t *testing.T) {
	now := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:new-provider-identity",
			Provider: "modrinth",
			ProjectID: "new-provider-identity",
			Name: "Explicit Install",
			Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Intent: "install",
			Target: &updatecheck.Release{
				ID: "v1",
				Number: "1.0.0",
				Filename: "shared.jar",
				URL: "https://cdn.example/shared.jar",
				SHA512: "identical-bytes",
			},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: now, ServerModsPath: "mods"},
		Mods: []management.Mod{{
			ID: "curseforge:existing",
			Provider: "curseforge",
			ProjectID: "existing",
			Name: "Existing Owner",
			Filename: "shared.jar",
			Management: "managed",
			Deployment: inventory.LocationServer,
			Path: "mods/shared.jar",
			SHA512: "identical-bytes",
		}},
	}

	plan, err := Build([]string{"modrinth:new-provider-identity"}, report, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusBlocked {
		t.Fatalf("explicit install onto occupied path must block: %+v", plan)
	}
	found := false
	for _, blocker := range plan.Blockers {
		if blocker.Code == "target_path_occupied" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing occupied-target blocker: %+v", plan.Blockers)
	}
	if len(plan.Changes) != 1 || len(plan.Changes[0].Operations) != 1 {
		t.Fatalf("explicit install operation was incorrectly coalesced: %+v", plan.Changes)
	}
}


func TestBuildMovesClientArtifactBetweenAutoModpackGroups(t *testing.T) {
	now := time.Date(2026, 10, 6, 3, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:visual",
			Provider: "modrinth",
			ProjectID: "visual",
			Name: "Visual Mod",
			Deployment: inventory.LocationClient,
			AutoModpackGroup: "performance",
			Classification: updatecheck.ClassificationReview,
			Installed: updatecheck.Release{ID: "v1", Number: "1.0"},
			Target: &updatecheck.Release{
				ID: "v2", Number: "2.0", Filename: "visual-v2.jar",
				URL: "https://cdn.example/visual.jar", SHA512: "target-sha",
			},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{
			GeneratedAt: now,
			ClientModsPath: inventory.DefaultClientModsPath,
			ClientGroupModsPaths: map[string]string{
				"main": inventory.DefaultClientModsPath,
				"performance": "automodpack/host-modpack/performance/mods",
			},
		},
		Mods: []management.Mod{{
			ID: "modrinth:visual",
			Provider: "modrinth",
			ProjectID: "visual",
			Name: "Visual Mod",
			Management: "managed",
			Deployment: inventory.LocationClient,
			AutoModpackGroup: "main",
			Path: "automodpack/host-modpack/main/mods/visual-v1.jar",
			SHA512: "current-sha",
		}},
	}

	plan, err := Build([]string{"modrinth:visual"}, report, snapshot, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("changes = %+v", plan.Changes)
	}
	change := plan.Changes[0]
	if change.Artifact.AutoModpackGroup != "performance" {
		t.Fatalf("artifact group = %q", change.Artifact.AutoModpackGroup)
	}
	if len(change.Operations) != 1 {
		t.Fatalf("operations = %+v", change.Operations)
	}
	op := change.Operations[0]
	if op.CurrentPath != "automodpack/host-modpack/main/mods/visual-v1.jar" {
		t.Fatalf("current path = %q", op.CurrentPath)
	}
	if op.TargetPath != "automodpack/host-modpack/performance/mods/visual-v2.jar" {
		t.Fatalf("target path = %q", op.TargetPath)
	}
}


func TestBuildReconcilesDependencyThatAppearedAfterResolution(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:main", Provider: "modrinth", ProjectID: "main", Name: "Main",
			Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationReview,
			Installed: updatecheck.Release{ID: "main-old"},
			Target: &updatecheck.Release{
				ID: "main-new", Filename: "main-new.jar",
				URL: "https://cdn.example/main.jar", SHA512: "main-new-hash",
			},
			Dependencies: []updatecheck.Dependency{{
				Provider: "modrinth", ProjectID: "kotlin", VersionID: "kotlin-v1",
				Name: "Kotlin for Forge", Type: "required", Action: "add",
				Deployment: inventory.LocationServer,
				Target: &updatecheck.Release{
					ID: "kotlin-v1", Filename: "kotlin.jar",
					URL: "https://cdn.example/kotlin.jar", SHA512: "kotlin-exact",
				},
			}},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: now, ServerModsPath: "mods"},
		Mods: []management.Mod{
			{
				ID: "modrinth:main", Provider: "modrinth", ProjectID: "main",
				Name: "Main", Management: "managed", Deployment: inventory.LocationServer,
				Path: "mods/main-old.jar", SHA512: "main-old-hash",
			},
			{
				ID: "modrinth:kotlin", Provider: "modrinth", ProjectID: "kotlin",
				Name: "Kotlin for Forge", Management: "managed", Deployment: inventory.LocationServer,
				Path: "mods/kotlin.jar", SHA512: "kotlin-exact",
			},
		},
	}

	plan, err := Build([]string{"modrinth:main"}, report, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusReady {
		t.Fatalf("stale dependency addition should reconcile, blockers=%+v", plan.Blockers)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("satisfied dependency should not create a second change: %+v", plan.Changes)
	}
	for _, blocker := range plan.Blockers {
		if blocker.Code == "dependency_state_changed" {
			t.Fatalf("legacy dependency_state_changed blocker survived: %+v", blocker)
		}
	}
}

func TestBuildAllowsGroupedModToUseCommonServerDependency(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:visual", Provider: "modrinth", ProjectID: "visual", Name: "Visual Mod",
			Deployment: inventory.LocationClient, AutoModpackGroup: "visual-client-mods",
			Classification: updatecheck.ClassificationReview,
			Installed: updatecheck.Release{ID: "visual-old"},
			Target: &updatecheck.Release{
				ID: "visual-new", Filename: "visual-new.jar",
				URL: "https://cdn.example/visual.jar", SHA512: "visual-new-hash",
			},
			Dependencies: []updatecheck.Dependency{{
				Provider: "modrinth", ProjectID: "library", Name: "Shared Library",
				Type: "required", Action: "add", Deployment: inventory.LocationClient,
				AutoModpackGroup: "visual-client-mods",
				Target: &updatecheck.Release{
					ID: "latest", Filename: "library.jar",
					URL: "https://cdn.example/library.jar", SHA512: "library-exact",
				},
			}},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{
			GeneratedAt: now,
			ServerModsPath: "mods",
			ClientGroupModsPaths: map[string]string{
				"visual-client-mods": "automodpack/host-modpack/visual-client-mods/mods",
			},
		},
		Mods: []management.Mod{
			{
				ID: "modrinth:visual", Provider: "modrinth", ProjectID: "visual",
				Name: "Visual Mod", Management: "managed", Deployment: inventory.LocationClient,
				AutoModpackGroup: "visual-client-mods",
				Path: "automodpack/host-modpack/visual-client-mods/mods/visual-old.jar",
				SHA512: "visual-old-hash",
			},
			{
				ID: "modrinth:library", Provider: "modrinth", ProjectID: "library",
				Name: "Shared Library", Management: "managed", Deployment: inventory.LocationServer,
				Path: "mods/library.jar", SHA512: "library-exact",
			},
		},
	}

	plan, err := Build([]string{"modrinth:visual"}, report, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusReady {
		t.Fatalf("server/common dependency should satisfy grouped mod: %+v", plan.Blockers)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("dependency was incorrectly duplicated into AutoModpack group: %+v", plan.Changes)
	}
}

func TestBuildUsesCrossProviderModrinthIdentityForDependency(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:main", Provider: "modrinth", ProjectID: "main", Name: "Main",
			Deployment: inventory.LocationClient, AutoModpackGroup: "main",
			Classification: updatecheck.ClassificationReview,
			Target: &updatecheck.Release{
				ID: "main-new", Filename: "main.jar",
				URL: "https://cdn.example/main.jar", SHA512: "main-new",
			},
			Dependencies: []updatecheck.Dependency{{
				Provider: "modrinth", ProjectID: "library-modrinth", Name: "Library",
				Type: "required", Action: "add", Deployment: inventory.LocationClient,
				Target: &updatecheck.Release{
					ID: "provider-latest", Filename: "library.jar",
					URL: "https://cdn.example/library.jar", SHA512: "different-provider-latest",
				},
			}},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{
			GeneratedAt: now,
			ServerModsPath: "mods",
			Mods: []inventory.ModFile{{
				Location: inventory.LocationServer,
				Path: "mods/library.jar",
				SHA512: "installed-library",
				Modrinth: &inventory.ModrinthMatch{
					ProjectID: "library-modrinth",
					VersionID: "installed-version",
				},
			}},
		},
		Mods: []management.Mod{
			{
				ID: "modrinth:main", Provider: "modrinth", ProjectID: "main",
				Name: "Main", Management: "managed", Deployment: inventory.LocationClient,
				Path: "automodpack/host-modpack/main/mods/main-old.jar", SHA512: "main-old",
			},
			{
				ID: "curseforge:123", Provider: "curseforge", ProjectID: "123",
				Name: "Library", Management: "managed", Deployment: inventory.LocationServer,
				Path: "mods/library.jar", SHA512: "installed-library",
			},
		},
	}

	plan, err := Build([]string{"modrinth:main"}, report, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusReady || len(plan.Changes) != 1 {
		t.Fatalf(
			"cross-provider managed alias should satisfy an unversioned dependency: changes=%+v blockers=%+v",
			plan.Changes,
			plan.Blockers,
		)
	}
}

func TestFindingsExposeAffectedModAndPath(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:main", Provider: "modrinth", ProjectID: "main", Name: "Main",
			Deployment: inventory.LocationServer,
			Classification: updatecheck.ClassificationReview,
			Target: &updatecheck.Release{
				ID: "new", Filename: "main.jar",
				URL: "https://cdn.example/main.jar", SHA512: "new",
			},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{GeneratedAt: now, ServerModsPath: "mods"},
		Mods: []management.Mod{{
			ID: "modrinth:main", Provider: "modrinth", ProjectID: "main",
			Name: "Main", Management: "managed", Deployment: inventory.LocationServer,
			Path: "mods/main-old.jar", SHA512: "old",
		}},
	}
	plan, err := Build([]string{"modrinth:main"}, report, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) == 0 {
		t.Fatalf("expected review warning")
	}
	if plan.Warnings[0].Name == "" || plan.Warnings[0].Path == "" {
		t.Fatalf("warning lacks mod/file context: %+v", plan.Warnings[0])
	}
}


func TestBuildAllowsPurePlacementMoveWithoutDownloadURL(t *testing.T) {
	now := time.Date(2026, 10, 7, 14, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:example",
			Provider: "modrinth",
			ProjectID: "example",
			Name: "Example",
			Deployment: inventory.LocationClient,
			AutoModpackGroup: "visual-client-mods",
			Classification: updatecheck.ClassificationSafe,
			Intent: "placement",
			Installed: updatecheck.Release{
				ID: "v1", Number: "1.0", Filename: "example.jar", SHA512: "same-bytes",
			},
			Target: &updatecheck.Release{
				ID: "v1", Number: "1.0", Filename: "example.jar", SHA512: "same-bytes",
			},
		}},
	}
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{
			GeneratedAt: now,
			ServerModsPath: "mods",
			ClientGroupModsPaths: map[string]string{
				"visual-client-mods": "automodpack/host-modpack/visual-client-mods/mods",
			},
		},
		Mods: []management.Mod{{
			ID: "modrinth:example",
			Provider: "modrinth",
			ProjectID: "example",
			Name: "Example",
			Management: "managed",
			Deployment: inventory.LocationServer,
			Path: "mods/example.jar",
			Filename: "example.jar",
			SHA512: "same-bytes",
		}},
	}

	plan, err := Build([]string{"modrinth:example"}, report, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != StatusReady {
		t.Fatalf("pure move should not require a provider URL: %+v", plan.Blockers)
	}
	if len(plan.Changes) != 1 || len(plan.Changes[0].Operations) != 1 {
		t.Fatalf("unexpected move changes: %+v", plan.Changes)
	}
	op := plan.Changes[0].Operations[0]
	if op.Action != "replace" ||
		op.CurrentPath != "mods/example.jar" ||
		op.TargetPath != "automodpack/host-modpack/visual-client-mods/mods/example.jar" {
		t.Fatalf("unexpected move operation: %+v", op)
	}
}
