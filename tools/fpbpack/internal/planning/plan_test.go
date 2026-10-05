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
