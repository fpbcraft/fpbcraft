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
		if blocker.Code == "dependency_artifact_not_resolved" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected dependency blocker: %+v", plan.Blockers)
	}
}
