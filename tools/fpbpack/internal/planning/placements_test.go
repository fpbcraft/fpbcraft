package planning

import (
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
	"testing"
	"time"
)

func dualPlacementSnapshot() management.Snapshot {
	return management.Snapshot{Mods: []management.Mod{
		{ID: "modrinth:async", Provider: "modrinth", ProjectID: "async", Management: "managed", Deployment: inventory.LocationClient, AutoModpackGroup: "main", Path: "automodpack/host-modpack/main/mods/old.jar", Filename: "old.jar", SHA512: "old"},
		{ID: "modrinth:async", Provider: "modrinth", ProjectID: "async", Management: "managed", Deployment: inventory.LocationServer, Path: "mods/old.jar", Filename: "old.jar", SHA512: "old"},
	}}
}
func TestUpdateReplacesEveryIntentionalPlacement(t *testing.T) {
	candidate := updatecheck.Candidate{Key: "modrinth:async", Provider: "modrinth", ProjectID: "async", Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationSafe, Target: &updatecheck.Release{ID: "v2", Filename: "new.jar", URL: "https://example.test/new.jar", SHA512: "new"}}
	plan, err := Build([]string{candidate.Key}, updatecheck.Report{Candidates: []updatecheck.Candidate{candidate}}, dualPlacementSnapshot(), time.Now())
	if err != nil || plan.Status != StatusReady {
		t.Fatalf("%v %+v", err, plan)
	}
	ops := plan.Changes[0].Operations
	if len(ops) != 2 || ops[0].TargetPath != "automodpack/host-modpack/main/mods/new.jar" || ops[1].TargetPath != "mods/new.jar" {
		t.Fatalf("not all placements updated: %+v", ops)
	}
}
func TestRemovalAddressesSelectedPaths(t *testing.T) {
	for _, paths := range [][]string{{"mods/old.jar"}, {"mods/old.jar", "automodpack/host-modpack/main/mods/old.jar"}} {
		candidate := updatecheck.Candidate{Key: "modrinth:async", Provider: "modrinth", ProjectID: "async", Classification: updatecheck.ClassificationReview, Intent: "remove", SourcePath: paths[0], SourcePaths: paths}
		plan, err := Build([]string{candidate.Key}, updatecheck.Report{Candidates: []updatecheck.Candidate{candidate}}, dualPlacementSnapshot(), time.Now())
		if err != nil || plan.Status != StatusReady || len(plan.Changes[0].Operations) != len(paths) {
			t.Fatalf("%v %+v", err, plan)
		}
		if plan.Changes[0].Operations[0].CurrentPath != paths[0] {
			t.Fatalf("wrong removal: %+v", plan.Changes)
		}
	}
}
func TestMoveOntoIdenticalManagedCopyRemovesOnlySource(t *testing.T) {
	candidate := updatecheck.Candidate{Key: "modrinth:async", Provider: "modrinth", ProjectID: "async", Intent: "placement", SourcePath: "automodpack/host-modpack/main/mods/old.jar", Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationSafe, Target: &updatecheck.Release{ID: "v1", Filename: "old.jar", SHA512: "old"}}
	plan, err := Build([]string{candidate.Key}, updatecheck.Report{Candidates: []updatecheck.Candidate{candidate}}, dualPlacementSnapshot(), time.Now())
	if err != nil || plan.Status != StatusReady {
		t.Fatalf("%v %+v", err, plan)
	}
	op := plan.Changes[0].Operations[0]
	if op.Action != "remove" || op.CurrentPath != candidate.SourcePath || op.TargetPath != candidate.SourcePath {
		t.Fatalf("unexpected repair: %+v", op)
	}
}
func TestMoveDoesNotOverwriteDifferentTarget(t *testing.T) {
	snapshot := dualPlacementSnapshot()
	snapshot.Mods[1].SHA512 = "different"
	candidate := updatecheck.Candidate{Key: "modrinth:async", Provider: "modrinth", ProjectID: "async", Intent: "placement", SourcePath: snapshot.Mods[0].Path, Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationSafe, Target: &updatecheck.Release{ID: "v1", Filename: "old.jar", SHA512: "old"}}
	plan, err := Build([]string{candidate.Key}, updatecheck.Report{Candidates: []updatecheck.Candidate{candidate}}, snapshot, time.Now())
	if err != nil || plan.Status != StatusBlocked {
		t.Fatalf("must reject occupied target: %v %+v", err, plan)
	}
}

func TestDependencyUpdateRetainsCanonicalArtifactIdentity(t *testing.T) {
	snapshot := dualPlacementSnapshot()
	for i := range snapshot.Mods {
		snapshot.Mods[i].ID = "modrinth:async#canonical"
		snapshot.Mods[i].Aliases = []string{"modrinth:async#retired"}
	}
	snapshot.Mods = append(snapshot.Mods, management.Mod{ID: "modrinth:parent", Provider: "modrinth", ProjectID: "parent", Deployment: inventory.LocationServer, Path: "mods/parent-old.jar", SHA512: "parent-old"})
	candidate := updatecheck.Candidate{Key: "modrinth:parent", Provider: "modrinth", ProjectID: "parent", Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationReview, Target: &updatecheck.Release{ID: "parent-v2", Filename: "parent-new.jar", URL: "https://example.test/parent.jar", SHA512: "parent-new"}, Dependencies: []updatecheck.Dependency{{Provider: "modrinth", ProjectID: "async", Action: "update", Type: "required", Deployment: inventory.LocationServer, Target: &updatecheck.Release{ID: "async-v2", Filename: "async-new.jar", URL: "https://example.test/async.jar", SHA512: "async-new"}}}}
	plan, err := Build([]string{candidate.Key}, updatecheck.Report{Candidates: []updatecheck.Candidate{candidate}}, snapshot, time.Now())
	if err != nil || plan.Status != StatusReady {
		t.Fatalf("%v %+v", err, plan)
	}
	for _, change := range plan.Changes {
		if change.DependencyDriven {
			if change.CandidateKey != "modrinth:async#canonical" || len(change.Operations) != 2 {
				t.Fatalf("dependency lost canonical identity/placements: %+v", change)
			}
			return
		}
	}
	t.Fatal("dependency update missing")
}

func TestVersionUpdateWithMoveCoalescesTargetWrites(t *testing.T) {
	snapshot := dualPlacementSnapshot()
	candidate := updatecheck.Candidate{Key: "modrinth:async", Provider: "modrinth", ProjectID: "async", Intent: "version", SourcePath: snapshot.Mods[0].Path, Deployment: inventory.LocationServer, Classification: updatecheck.ClassificationSafe, Target: &updatecheck.Release{ID: "v2", Filename: "new.jar", URL: "https://example.test/new.jar", SHA512: "new"}}
	plan, err := Build([]string{candidate.Key}, updatecheck.Report{Candidates: []updatecheck.Candidate{candidate}}, snapshot, time.Now())
	if err != nil || plan.Status != StatusReady {
		t.Fatalf("%v %+v", err, plan)
	}
	writes, removes := 0, 0
	for _, op := range plan.Changes[0].Operations {
		if op.Action == "remove" {
			removes++
		} else {
			writes++
			if op.CurrentPath != "mods/old.jar" || op.TargetPath != "mods/new.jar" {
				t.Fatalf("wrong destination writer: %+v", op)
			}
		}
	}
	if writes != 1 || removes != 1 {
		t.Fatalf("expected one writer and one redundant copy removal: %+v", plan.Changes[0].Operations)
	}
}
