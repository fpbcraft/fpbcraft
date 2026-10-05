package doctor

import (
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestAnalyzeDetectsManagedDriftAndIgnoresPinnedChanges(t *testing.T) {
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		Mods: []inventory.ModFile{
			{
				Location: inventory.LocationServer,
				Path:     "mods/create.jar",
				Filename: "create.jar",
				SHA512:   "new-create",
				Metadata: []inventory.ModMetadata{{Name: "Create"}},
			},
			{
				Location: inventory.LocationServer,
				Path:     "mods/custom.jar",
				Filename: "custom.jar",
				SHA512:   "changed-custom",
			},
			{
				Location: inventory.LocationClient,
				Path:     "automodpack/host-modpack/main/mods/new.jar",
				Filename: "new.jar",
				SHA512:   "new-file",
			},
		},
	}
	cat := catalog.Report{
		InventorySchema: inventory.SchemaVersion,
		Managed: []catalog.Entry{{
			Provider:  "modrinth",
			ProjectID: "create",
			Name:      "Create",
			SHA512:    "old-create",
			SourcePaths: []catalog.Source{{
				Location: inventory.LocationServer,
				Path:     "mods/create.jar",
			}},
		}},
		Pinned: []catalog.PinnedArtifact{{
			SHA512:   "old-custom",
			Filename: "custom.jar",
			Sources: []catalog.Source{{
				Location: inventory.LocationServer,
				Path:     "mods/custom.jar",
			}},
		}},
	}

	report := Analyze(inv, cat)

	assertFinding(t, report, "managed_artifact_replaced")
	assertFinding(t, report, "external_artifact_added")
	assertFinding(t, report, "unmanaged_artifacts")
	if hasFinding(report, "managed_artifact_missing") {
		t.Fatal("did not expect managed_artifact_missing")
	}
	if report.Summary.Blocking != 2 {
		t.Fatalf("blocking = %d, want 2", report.Summary.Blocking)
	}
}

func TestAnalyzeDetectsMovedManagedArtifact(t *testing.T) {
	inv := inventory.Inventory{
		SchemaVersion: inventory.SchemaVersion,
		Mods: []inventory.ModFile{{
			Location: inventory.LocationClient,
			Path:     "automodpack/host-modpack/main/mods/create.jar",
			Filename: "create.jar",
			SHA512:   "same",
		}},
	}
	cat := catalog.Report{
		InventorySchema: inventory.SchemaVersion,
		Managed: []catalog.Entry{{
			Name:   "Create",
			SHA512: "same",
			SourcePaths: []catalog.Source{{
				Location: inventory.LocationServer,
				Path:     "mods/create.jar",
			}},
		}},
	}

	report := Analyze(inv, cat)
	assertFinding(t, report, "managed_artifact_moved")
}

func TestAnalyzeReportsCatalogProblemsAsActionable(t *testing.T) {
	inv := inventory.Inventory{SchemaVersion: inventory.SchemaVersion}
	cat := catalog.Report{
		InventorySchema: inventory.SchemaVersion,
		Unresolved:      []catalog.Unresolved{{Filename: "unknown.jar"}},
		Conflicts:       []catalog.Conflict{{Provider: "modrinth", ProjectID: "create", Files: []catalog.ConflictFile{{}, {}}}},
		Placement:       []catalog.PlacementWarning{{Filename: "client.jar", Environment: "client", Deployment: inventory.LocationServer}},
	}

	report := Analyze(inv, cat)

	assertFinding(t, report, "unresolved_artifact")
	assertFinding(t, report, "project_version_conflict")
	assertFinding(t, report, "placement_warning")
	if report.Summary.Actionable != 3 {
		t.Fatalf("actionable = %d, want 3", report.Summary.Actionable)
	}
}

func assertFinding(t *testing.T, report Report, code string) {
	t.Helper()
	if !hasFinding(report, code) {
		t.Fatalf("missing finding %q: %#v", code, report.Findings)
	}
}

func hasFinding(report Report, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
