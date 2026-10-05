package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestDiscoverCurseForgeCandidate(t *testing.T) {
	currentDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	targetDate := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing API key")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/mods/123":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": 123,
				"name": "Curse Example",
				"slug": "curse-example",
				"links": map[string]any{"websiteUrl": "https://www.curseforge.com/minecraft/mc-mods/curse-example"},
				"logo": map[string]any{"thumbnailUrl": "https://example.invalid/icon.png"},
			}})
		case "/mods/123/files/10":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": curseFileJSON(
				10, 123, "1.0.0", "old.jar", currentDate, "oldsha1", "",
			)})
		case "/mods/123/files":
			if r.URL.Query().Get("gameVersion") != "1.21.1" || r.URL.Query().Get("modLoaderType") != "6" {
				t.Fatalf("unexpected CurseForge filters: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{
				curseFileJSON(11, 123, "1.1.0", "new.jar", targetDate, "newsha1", server.URL+"/download/11"),
			}})
		case "/mods/123/files/11/changelog":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "<p>Fixed the important thing.</p>"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	report := Discover(context.Background(), catalog.Report{
		Managed: []catalog.Entry{{
			Provider: "curseforge",
			ProjectID: "123",
			FileID: 10,
			Name: "Curse Example",
			Filename: "old.jar",
			SHA512: "installed-sha512",
			Side: "both",
			Deployment: inventory.LocationServer,
		}},
	}, Options{
		Minecraft: "1.21.1",
		Loader: "neoforge",
		CurseForgeBaseURL: server.URL,
		CurseForgeAPIKey: "test-key",
		HTTPClient: server.Client(),
	})

	if len(report.Candidates) != 1 {
		t.Fatalf("candidates = %d", len(report.Candidates))
	}
	candidate := report.Candidates[0]
	if candidate.Classification != ClassificationSafe {
		t.Fatalf("classification = %s reasons=%+v", candidate.Classification, candidate.Reasons)
	}
	if candidate.Target == nil || candidate.Target.ID != "11" || candidate.Target.SHA1 != "newsha1" {
		t.Fatalf("unexpected target: %+v", candidate.Target)
	}
	if candidate.ProjectURL != "https://www.curseforge.com/minecraft/mc-mods/curse-example" {
		t.Fatalf("project URL = %q", candidate.ProjectURL)
	}
	if len(candidate.Changelogs) != 1 || candidate.Changelogs[0].Body != "Fixed the important thing." {
		t.Fatalf("unexpected changelog: %+v", candidate.Changelogs)
	}
}

func TestDiscoverCurseForgeWithoutKeyIsExplicitlyBlocked(t *testing.T) {
	report := Discover(context.Background(), catalog.Report{
		Managed: []catalog.Entry{{
			Provider: "curseforge",
			ProjectID: "123",
			FileID: 10,
			Name: "Curse Example",
		}},
	}, Options{Minecraft: "1.21.1", Loader: "neoforge"})

	if report.Candidates[0].Classification != ClassificationBlocked {
		t.Fatalf("classification = %s", report.Candidates[0].Classification)
	}
	if len(report.Candidates[0].Reasons) == 0 || report.Candidates[0].Reasons[0].Code != "curseforge_api_key_missing" {
		t.Fatalf("unexpected reasons: %+v", report.Candidates[0].Reasons)
	}
}

func curseFileJSON(
	id int,
	modID int,
	displayName string,
	fileName string,
	fileDate time.Time,
	sha1 string,
	downloadURL string,
) map[string]any {
	return map[string]any{
		"id": id,
		"modId": modID,
		"isAvailable": true,
		"displayName": displayName,
		"fileName": fileName,
		"releaseType": 1,
		"fileDate": fileDate.Format(time.RFC3339),
		"fileLength": 1024,
		"downloadUrl": downloadURL,
		"gameVersions": []string{"1.21.1", "NeoForge"},
		"hashes": []any{map[string]any{"value": sha1, "algo": 1}},
		"dependencies": []any{},
	}
}

func TestDiscoverCurseForgeManualDownloadCandidate(t *testing.T) {
	currentDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	targetDate := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/mods/123":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": 123,
				"name": "Restricted",
				"slug": "restricted",
				"links": map[string]any{"websiteUrl": "https://www.curseforge.com/minecraft/mc-mods/restricted"},
			}})
		case "/mods/123/files/10":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": curseFileJSON(
				10, 123, "1.0.0", "old.jar", currentDate, "oldsha1", "",
			)})
		case "/mods/123/files":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{
				curseFileJSON(11, 123, "1.1.0", "restricted.jar", targetDate, "newsha1", ""),
			}})
		case "/mods/123/files/11/download-url":
			http.Error(w, "third-party distribution disabled", http.StatusForbidden)
		case "/mods/123/files/11/changelog":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "<p>Manual release.</p>"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	report := Discover(context.Background(), catalog.Report{
		Managed: []catalog.Entry{{
			Provider: "curseforge",
			ProjectID: "123",
			FileID: 10,
			Name: "Restricted",
			Filename: "old.jar",
			SHA512: "installed-sha512",
			Side: "both",
			Deployment: inventory.LocationServer,
		}},
	}, Options{
		Minecraft: "1.21.1",
		Loader: "neoforge",
		Mode: RefreshModeInteractive,
		CurseForgeBaseURL: server.URL,
		CurseForgeAPIKey: "test-key",
		HTTPClient: server.Client(),
	})

	candidate := report.Candidates[0]
	if candidate.Classification != ClassificationReview {
		t.Fatalf("classification = %s reasons=%+v", candidate.Classification, candidate.Reasons)
	}
	if candidate.Target == nil || !candidate.Target.ManualDownload {
		t.Fatalf("expected manual-download target: %+v", candidate.Target)
	}
	wantURL := "https://www.curseforge.com/minecraft/mc-mods/restricted/files/11"
	if candidate.Target.ManualURL != wantURL {
		t.Fatalf("manual URL = %q, want %q", candidate.Target.ManualURL, wantURL)
	}
	found := false
	for _, reason := range candidate.Reasons {
		if reason.Code == "manual_download_required" {
			found = true
		}
	}
	if !found {
		t.Fatalf("manual-download reason missing: %+v", candidate.Reasons)
	}
}
