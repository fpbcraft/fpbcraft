package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func TestProposedLibraryUpgradeChecksUnselectedProviderDependent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/versions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "parent-v1", "project_id": "parent", "dependencies": []map[string]any{{"project_id": "library", "version_id": "library-v1", "dependency_type": "required"}}},
			{"id": "library-v2", "project_id": "library", "dependencies": []any{}},
		})
	}))
	defer server.Close()
	svc := &Service{options: Options{ModrinthBaseURL: server.URL}, state: State{Catalog: catalog.Report{Managed: []catalog.Entry{
		{Provider: "modrinth", ProjectID: "parent", VersionID: "parent-v1", Name: "Unselected parent", SHA512: "parent", SourcePaths: []catalog.Source{{Path: "mods/parent.jar"}}},
		{Provider: "modrinth", ProjectID: "library", VersionID: "library-v1", Name: "Library", SHA512: "old", SourcePaths: []catalog.Source{{Path: "mods/library-old.jar"}}},
	}}}}
	plan := planning.Plan{Status: planning.StatusReady, Changes: []planning.Change{{CandidateKey: "modrinth:library", Target: updatecheck.Release{ID: "library-v2"}, Artifact: planning.Artifact{Provider: "modrinth", ProjectID: "library", Filename: "library-new.jar", SHA512: "new", Deployment: "server"}, Operations: []planning.FileOperation{{Action: "replace", CurrentPath: "mods/library-old.jar", TargetPath: "mods/library-new.jar"}}}}}
	svc.validateProviderRequirements(context.Background(), &plan)
	if len(plan.Blockers) != 1 || plan.Blockers[0].Code != "final_provider_dependency_conflict" || plan.Blockers[0].Name != "Unselected parent" {
		t.Fatalf("unselected exact requirement was ignored: %+v", plan.Blockers)
	}
}

func TestProviderDependencyRequiresEffectiveClientGroup(t *testing.T) {
	owner := catalog.Entry{SourcePaths: []catalog.Source{{Location: "client", Group: "main"}}}
	library := catalog.Entry{VersionID: "v1", SourcePaths: []catalog.Source{{Location: "client", Group: "optional"}}}
	satisfied, _, _ := providerDependencyAvailability(owner, []catalog.Entry{library}, "v1")
	if satisfied {
		t.Fatal("an optional-only exact provider dependency cannot satisfy main")
	}
	library.SourcePaths[0].Location = "server"
	satisfied, _, uncertain := providerDependencyAvailability(owner, []catalog.Entry{library}, "v1")
	if !satisfied || !uncertain {
		t.Fatal("server/common delivery must be explicitly unverified")
	}
	library.SourcePaths[0] = catalog.Source{Location: "client", Group: "main"}
	satisfied, _, uncertain = providerDependencyAvailability(owner, []catalog.Entry{library}, "v1")
	if !satisfied || uncertain {
		t.Fatal("main library should be visible")
	}
}
