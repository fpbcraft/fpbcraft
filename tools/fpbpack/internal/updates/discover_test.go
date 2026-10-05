package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestDiscoverClassifiesCompatibleUpdateAndDependencyChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/projects":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "p1", "title": "Example Mod", "slug": "example-mod", "icon_url": "https://cdn.example/icon.png"},
				{"id": "p2", "title": "Dependency", "slug": "dependency"},
			})
		case r.URL.Path == "/project/p1/version":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				versionJSON("v3", "p1", "3.0.0", "release", "2026-03-01T00:00:00Z", []string{"1.22"}, []string{"neoforge"}, nil),
				versionJSON("v2", "p1", "2.0.0", "release", "2026-02-01T00:00:00Z", []string{"1.21.1"}, []string{"neoforge"}, []map[string]any{{"project_id": "p2", "version_id": "dep-new", "dependency_type": "required"}}),
				versionJSON("v1", "p1", "1.9.0", "release", "2026-01-01T00:00:00Z", []string{"1.21.1"}, []string{"neoforge"}, nil),
			})
		case r.URL.Path == "/project/p2/version":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				versionJSON("dep-old", "p2", "1.0.0", "release", "2026-01-01T00:00:00Z", []string{"1.21.1"}, []string{"neoforge"}, nil),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	report := Discover(context.Background(), catalog.Report{Managed: []catalog.Entry{
		modrinthEntry("p1", "v1", "Example Mod"),
		modrinthEntry("p2", "dep-old", "Dependency"),
	}}, Options{Minecraft: "1.21.1", Loader: "neoforge", ModrinthBaseURL: server.URL, HTTPClient: server.Client()})

	if report.Summary.Review != 1 || report.Summary.UpToDate != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	var candidate Candidate
	for _, item := range report.Candidates {
		if item.ProjectID == "p1" {
			candidate = item
		}
	}
	if candidate.Target == nil || candidate.Target.ID != "v2" {
		t.Fatalf("unexpected target: %+v", candidate.Target)
	}
	if candidate.Classification != ClassificationReview {
		t.Fatalf("classification = %q, want review; candidate=%+v", candidate.Classification, candidate)
	}
	if candidate.Name != "Example Mod" || !strings.Contains(candidate.ProjectURL, "example-mod") || candidate.IconURL == "" {
		t.Fatalf("project metadata not applied: %+v", candidate)
	}
	if len(candidate.Dependencies) != 1 || candidate.Dependencies[0].Action != "update" {
		t.Fatalf("unexpected dependency classification: %+v", candidate.Dependencies)
	}
	if len(candidate.Rejected) != 1 || candidate.Rejected[0].ID != "v3" {
		t.Fatalf("expected incompatible newer version to be preserved: %+v", candidate.Rejected)
	}
}

func TestDiscoverBlocksIncompatibleInstalledDependency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/projects":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "p1", "title": "Example"}, {"id": "p2", "title": "Conflict"}})
		case r.URL.Path == "/project/p1/version":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				versionJSON("v2", "p1", "1.1.0", "release", "2026-02-01T00:00:00Z", []string{"1.21.1"}, []string{"neoforge"}, []map[string]any{{"project_id": "p2", "dependency_type": "incompatible"}}),
				versionJSON("v1", "p1", "1.0.0", "release", "2026-01-01T00:00:00Z", []string{"1.21.1"}, []string{"neoforge"}, nil),
			})
		case r.URL.Path == "/project/p2/version":
			_ = json.NewEncoder(w).Encode([]map[string]any{versionJSON("p2-v1", "p2", "1.0.0", "release", "2026-01-01T00:00:00Z", []string{"1.21.1"}, []string{"neoforge"}, nil)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	report := Discover(context.Background(), catalog.Report{Managed: []catalog.Entry{
		modrinthEntry("p1", "v1", "Example"),
		modrinthEntry("p2", "p2-v1", "Conflict"),
	}}, Options{Minecraft: "1.21.1", Loader: "neoforge", ModrinthBaseURL: server.URL, HTTPClient: server.Client()})

	if report.Summary.Blocked != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	for _, candidate := range report.Candidates {
		if candidate.ProjectID == "p1" && candidate.Classification != ClassificationBlocked {
			t.Fatalf("expected p1 blocked, got %+v", candidate)
		}
	}
}

func TestDiscoverClassifiesPatchReleaseSafe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/projects" {
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "p1", "title": "Example"}})
			return
		}
		if r.URL.Path == "/project/p1/version" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				versionJSON("v2", "p1", "1.2.4", "release", "2026-02-01T00:00:00Z", []string{"1.21.1"}, []string{"neoforge"}, nil),
				versionJSON("v1", "p1", "1.2.3", "release", "2026-01-01T00:00:00Z", []string{"1.21.1"}, []string{"neoforge"}, nil),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	report := Discover(context.Background(), catalog.Report{Managed: []catalog.Entry{modrinthEntry("p1", "v1", "Example")}}, Options{
		Minecraft: "1.21.1", Loader: "neoforge", ModrinthBaseURL: server.URL, HTTPClient: server.Client(),
	})
	if report.Summary.Safe != 1 || report.Candidates[0].Classification != ClassificationSafe {
		t.Fatalf("expected safe update: %+v", report)
	}
}

func TestVersionMajor(t *testing.T) {
	tests := []struct {
		input string
		want  int
		ok    bool
	}{
		{"1.2.3", 1, true},
		{"v12.4", 12, true},
		{"release", 0, false},
	}
	for _, test := range tests {
		got, ok := versionMajor(test.input)
		if got != test.want || ok != test.ok {
			t.Fatalf("versionMajor(%q) = (%d, %v), want (%d, %v)", test.input, got, ok, test.want, test.ok)
		}
	}
}

func modrinthEntry(projectID, versionID, name string) catalog.Entry {
	return catalog.Entry{
		Provider: "modrinth", ProjectID: projectID, VersionID: versionID, Name: name,
		Filename: strings.ToLower(strings.ReplaceAll(name, " ", "-")) + ".jar",
		SHA512: projectID + "-hash", Side: "both", Deployment: inventory.LocationServer,
	}
}

func versionJSON(
	id, projectID, number, channel, published string,
	gameVersions, loaders []string,
	dependencies []map[string]any,
) map[string]any {
	publishedAt, _ := time.Parse(time.RFC3339, published)
	return map[string]any{
		"id": id,
		"project_id": projectID,
		"name": "Version " + number,
		"version_number": number,
		"version_type": channel,
		"status": "listed",
		"date_published": publishedAt,
		"game_versions": gameVersions,
		"loaders": loaders,
		"environment": "client_and_server",
		"dependencies": dependencies,
		"files": []map[string]any{{
			"hashes": map[string]string{"sha512": id + "-hash"},
			"url": "https://cdn.example/" + id + ".jar",
			"filename": id + ".jar",
			"primary": true,
		}},
	}
}
