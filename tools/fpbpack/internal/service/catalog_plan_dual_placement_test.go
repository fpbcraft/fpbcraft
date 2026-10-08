package service

import (
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func TestCatalogAfterPlanCoalescesSameBytesInTwoDeployments(t *testing.T) {
	hash := "same-release"
	plan := planning.Plan{Changes: []planning.Change{
		{
			CandidateKey: "modrinth:sparkweave",
			Name: "Sparkweave",
			Target: updatecheck.Release{ID: "v1"},
			Artifact: planning.Artifact{
				Provider: "modrinth", ProjectID: "sparkweave",
				Filename: "Sparkweave.jar", SHA512: hash,
				Deployment: "server",
			},
			Operations: []planning.FileOperation{{
				Action: "add", TargetPath: "mods/Sparkweave.jar", TargetSHA512: hash,
			}},
		},
		{
			CandidateKey: "modrinth:sparkweave",
			Name: "Sparkweave",
			Target: updatecheck.Release{ID: "v1"},
			Artifact: planning.Artifact{
				Provider: "modrinth", ProjectID: "sparkweave",
				Filename: "Sparkweave.jar", SHA512: hash,
				Deployment: "client", AutoModpackGroup: "main",
			},
			Operations: []planning.FileOperation{{
				Action: "add", TargetPath: "automodpack/host-modpack/main/mods/Sparkweave.jar", TargetSHA512: hash,
			}},
		},
	}}
	report, err := catalogAfterPlan(catalog.Report{
		SchemaVersion: catalog.ReportSchemaVersion,
		InventorySchema: inventory.SchemaVersion,
	}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Managed) != 1 || len(report.Managed[0].SourcePaths) != 2 {
		t.Fatalf("identical releases should have two paths, one identity: %+v", report.Managed)
	}
}
