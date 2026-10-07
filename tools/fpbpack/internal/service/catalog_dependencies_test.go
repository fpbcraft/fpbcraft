package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
)

func TestCurrentRequiredByUsesInstalledModrinthVersions(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/versions" {
			http.NotFound(w, r)
			return
		}
		var ids []string
		if err := json.Unmarshal([]byte(r.URL.Query().Get("ids")), &ids); err != nil {
			t.Fatalf("decode ids: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		versions := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			dependencies := []map[string]any{}
			projectID := "unrelated"
			if id == "dependent-v1" {
				projectID = "dependent"
				dependencies = append(dependencies, map[string]any{
					"project_id": "library",
					"dependency_type": "required",
				})
			}
			versions = append(versions, map[string]any{
				"id": id,
				"project_id": projectID,
				"name": id,
				"version_number": "1.0.0",
				"version_type": "release",
				"status": "listed",
				"game_versions": []string{"1.21.1"},
				"loaders": []string{"neoforge"},
				"dependencies": dependencies,
				"files": []any{},
			})
		}
		_ = json.NewEncoder(w).Encode(versions)
	}))
	defer server.Close()

	target := catalog.Entry{
		Provider: "modrinth",
		ProjectID: "library",
		VersionID: "library-v1",
		Name: "Library",
	}
	cat := catalog.Report{Managed: []catalog.Entry{
		target,
		{
			Provider: "modrinth",
			ProjectID: "dependent",
			VersionID: "dependent-v1",
			Name: "Dependent Mod",
		},
		{
			Provider: "modrinth",
			ProjectID: "unrelated",
			VersionID: "unrelated-v1",
			Name: "Unrelated Mod",
		},
	}}
	service := &Service{
		options: Options{
			ModrinthBaseURL: server.URL,
			Minecraft: "1.21.1",
			Loader: "neoforge",
		},
	}

	requiredBy, err := service.currentRequiredBy(context.Background(), target, cat)
	if err != nil {
		t.Fatal(err)
	}
	if len(requiredBy) != 1 || requiredBy[0] != "Dependent Mod" {
		t.Fatalf("required by = %+v", requiredBy)
	}
	if requests != 1 {
		t.Fatalf("provider requests = %d, want 1 batched request", requests)
	}
}


func TestCurrentRequiredByResolvesVersionOnlyDependenciesInBatch(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/versions" {
			http.NotFound(w, r)
			return
		}
		var ids []string
		if err := json.Unmarshal([]byte(r.URL.Query().Get("ids")), &ids); err != nil {
			t.Fatalf("decode ids: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		versions := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			version := map[string]any{
				"id": id,
				"name": id,
				"version_number": "1.0.0",
				"version_type": "release",
				"status": "listed",
				"game_versions": []string{"1.21.1"},
				"loaders": []string{"neoforge"},
				"dependencies": []any{},
				"files": []any{},
			}
			switch id {
			case "dependent-v1":
				version["project_id"] = "dependent"
				version["dependencies"] = []map[string]any{{
					"version_id": "library-v1",
					"dependency_type": "required",
				}}
			case "library-v1":
				version["project_id"] = "library"
			}
			versions = append(versions, version)
		}
		_ = json.NewEncoder(w).Encode(versions)
	}))
	defer server.Close()

	target := catalog.Entry{
		Provider: "modrinth",
		ProjectID: "library",
		VersionID: "library-v1",
		Name: "Library",
	}
	cat := catalog.Report{Managed: []catalog.Entry{
		target,
		{
			Provider: "modrinth",
			ProjectID: "dependent",
			VersionID: "dependent-v1",
			Name: "Dependent Mod",
		},
	}}
	service := &Service{options: Options{ModrinthBaseURL: server.URL}}

	requiredBy, err := service.currentRequiredBy(context.Background(), target, cat)
	if err != nil {
		t.Fatal(err)
	}
	if len(requiredBy) != 1 || requiredBy[0] != "Dependent Mod" {
		t.Fatalf("required by = %+v", requiredBy)
	}
	if requests != 2 {
		t.Fatalf("provider requests = %d, want 2 batched requests", requests)
	}
}

func TestCurrentRequiredByFailsClosedWhenInstalledMetadataUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	target := catalog.Entry{
		Provider: "modrinth",
		ProjectID: "library",
		VersionID: "library-v1",
		Name: "Library",
	}
	cat := catalog.Report{Managed: []catalog.Entry{
		target,
		{
			Provider: "modrinth",
			ProjectID: "dependent",
			VersionID: "dependent-v1",
			Name: "Dependent Mod",
		},
	}}
	service := &Service{
		options: Options{ModrinthBaseURL: server.URL},
	}

	if _, err := service.currentRequiredBy(context.Background(), target, cat); err == nil {
		t.Fatal("provider failure must block safe removal verification")
	}
}
