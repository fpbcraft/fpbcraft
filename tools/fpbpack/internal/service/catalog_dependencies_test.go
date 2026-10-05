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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/version/dependent-v1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "dependent-v1",
				"project_id": "dependent",
				"name": "Dependent 1.0",
				"version_number": "1.0.0",
				"version_type": "release",
				"status": "listed",
				"game_versions": []string{"1.21.1"},
				"loaders": []string{"neoforge"},
				"dependencies": []map[string]any{{
					"project_id": "library",
					"dependency_type": "required",
				}},
				"files": []any{},
			})
		default:
			http.NotFound(w, r)
		}
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
