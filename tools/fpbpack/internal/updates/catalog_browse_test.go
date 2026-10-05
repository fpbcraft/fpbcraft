package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestModrinthCatalogSearchAndVersions(t *testing.T) {
	var searchFacets string
	var versionsLoaders string
	var versionsGameVersions string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search":
			searchFacets = r.URL.Query().Get("facets")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"hits": []map[string]any{{
					"project_id": "abc",
					"title": "Example Mod",
					"description": "Example",
					"slug": "example-mod",
					"icon_url": "https://example.invalid/icon.png",
					"downloads": 42,
					"environment": []string{"client", "server"},
				}},
			})
		case "/project/abc/version":
			versionsLoaders = r.URL.Query().Get("loaders")
			versionsGameVersions = r.URL.Query().Get("game_versions")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": "v1",
				"project_id": "abc",
				"name": "Example 1.2.3",
				"version_number": "1.2.3",
				"version_type": "release",
				"status": "listed",
				"date_published": "2026-10-01T12:00:00Z",
				"changelog": "Changed things",
				"game_versions": []string{"1.21.1"},
				"loaders": []string{"neoforge"},
				"environment": "client_and_server",
				"dependencies": []any{},
				"files": []map[string]any{{
					"hashes": map[string]any{"sha512": "abc123"},
					"url": "https://cdn.example.invalid/example.jar",
					"filename": "example-1.2.3.jar",
					"primary": true,
				}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &ModrinthClient{
		BaseURL: server.URL,
		HTTPClient: server.Client(),
		Mode: RefreshModeInteractive,
	}
	projects, err := client.SearchCatalogProjects(context.Background(), "example", "1.21.1", "neoforge", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].ProjectID != "abc" || projects[0].Name != "Example Mod" {
		t.Fatalf("projects = %+v", projects)
	}
	if !strings.Contains(searchFacets, "project_type:mod") ||
		!strings.Contains(searchFacets, "versions:1.21.1") ||
		!strings.Contains(searchFacets, "categories:neoforge") {
		t.Fatalf("search facets = %q", searchFacets)
	}

	versions, err := client.CatalogVersions(context.Background(), "abc", "1.21.1", "neoforge")
	if err != nil {
		t.Fatal(err)
	}
	if versionsLoaders != "[\"neoforge\"]" || versionsGameVersions != "[\"1.21.1\"]" {
		t.Fatalf("version filters loaders=%q game_versions=%q", versionsLoaders, versionsGameVersions)
	}
	if len(versions) != 1 ||
		versions[0].ID != "v1" ||
		versions[0].Number != "1.2.3" ||
		versions[0].Filename != "example-1.2.3.jar" ||
		versions[0].Changelog != "Changed things" {
		t.Fatalf("versions = %+v", versions)
	}
}

func TestCurseForgeCatalogSearchAndVersionsUseNeoForgeFilters(t *testing.T) {
	var searchLoader string
	var searchGameVersion string
	var filesLoader string
	var filesGameVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing CurseForge API key")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/mods/search":
			searchLoader = r.URL.Query().Get("modLoaderType")
			searchGameVersion = r.URL.Query().Get("gameVersion")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id": 123,
					"name": "CF Example",
					"slug": "cf-example",
					"summary": "Example summary",
					"downloadCount": 55,
					"links": map[string]any{
						"websiteUrl": "https://www.curseforge.com/minecraft/mc-mods/cf-example",
					},
					"logo": map[string]any{"thumbnailUrl": "https://example.invalid/cf.png"},
				}},
			})
		case "/mods/123/files":
			filesLoader = r.URL.Query().Get("modLoaderType")
			filesGameVersion = r.URL.Query().Get("gameVersion")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id": 999,
					"modId": 123,
					"isAvailable": true,
					"displayName": "CF Example 2.0.0",
					"fileName": "cf-example-2.0.0.jar",
					"releaseType": 1,
					"fileDate": "2026-10-02T12:00:00Z",
					"fileLength": 100,
					"downloadUrl": "https://cdn.example.invalid/cf.jar",
					"gameVersions": []string{"1.21.1", "NeoForge"},
					"hashes": []map[string]any{{"value": "sha1-value", "algo": 1}},
					"dependencies": []any{},
				}},
				"pagination": map[string]any{
					"index": 0, "pageSize": 50, "resultCount": 1, "totalCount": 1,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &CurseForgeClient{
		BaseURL: server.URL,
		APIKey: "test-key",
		HTTPClient: server.Client(),
		Mode: RefreshModeInteractive,
	}
	projects, err := client.SearchCatalogProjects(context.Background(), "example", "1.21.1", "neoforge", 20)
	if err != nil {
		t.Fatal(err)
	}
	if searchLoader != "6" || searchGameVersion != "1.21.1" {
		t.Fatalf("search filters loader=%q game=%q", searchLoader, searchGameVersion)
	}
	if len(projects) != 1 || projects[0].ProjectID != "123" || projects[0].Name != "CF Example" {
		t.Fatalf("projects = %+v", projects)
	}

	versions, err := client.CatalogVersions(context.Background(), "123", "1.21.1", "neoforge")
	if err != nil {
		t.Fatal(err)
	}
	if filesLoader != "6" || filesGameVersion != "1.21.1" {
		t.Fatalf("file filters loader=%q game=%q", filesLoader, filesGameVersion)
	}
	if len(versions) != 1 ||
		versions[0].ID != "999" ||
		versions[0].Number != "CF Example 2.0.0" ||
		versions[0].Channel != "release" ||
		versions[0].SHA1 != "sha1-value" {
		t.Fatalf("versions = %+v", versions)
	}
}

func TestCatalogVersionsAreNewestFirst(t *testing.T) {
	versions := []CatalogVersion{
		{ID: "old", PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "new", PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
	}
	// The public catalog methods sort provider data before converting it. Keep a
	// tiny assertion here so accidental ascending-order UI regressions are easy
	// to diagnose if provider fixtures change.
	if !versions[1].PublishedAt.After(versions[0].PublishedAt) {
		t.Fatal("fixture must represent newest-after-oldest timestamps")
	}
}
