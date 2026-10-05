package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

func TestDiscoverVerifiedGitHubRelease(t *testing.T) {
	digestBytes := sha256.Sum256([]byte("new release asset"))
	digest := hex.EncodeToString(digestBytes[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/example/mod/releases" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id": 2,
				"tag_name": "v2",
				"name": "Version 2",
				"body": "New feature",
				"draft": false,
				"prerelease": false,
				"published_at": "2026-02-01T00:00:00Z",
				"assets": []map[string]any{{
					"id": 20,
					"name": "mod-v2.jar",
					"browser_download_url": "https://example.invalid/mod-v2.jar",
					"digest": "sha256:" + digest,
				}},
			},
			{
				"id": 1,
				"tag_name": "v1",
				"name": "Version 1",
				"body": "Old",
				"draft": false,
				"prerelease": false,
				"published_at": "2026-01-01T00:00:00Z",
				"assets": []map[string]any{{
					"id": 10,
					"name": "mod-v1.jar",
					"browser_download_url": "https://example.invalid/mod-v1.jar",
					"digest": "sha256:old",
				}},
			},
		})
	}))
	defer server.Close()

	report := Discover(context.Background(), catalog.Report{
		Managed: []catalog.Entry{{
			Provider: "github",
			ProjectID: "example/mod",
			Repository: "example/mod",
			Tag: "v1",
			Asset: "mod-v1.jar",
			Name: "Example",
			Filename: "mod-v1.jar",
			SHA512: "installed",
			Deployment: inventory.LocationServer,
		}},
	}, Options{
		Minecraft: "1.21.1",
		Loader: "neoforge",
		GitHubBaseURL: server.URL,
		HTTPClient: server.Client(),
	})

	if len(report.Candidates) != 1 {
		t.Fatalf("candidates = %d", len(report.Candidates))
	}
	candidate := report.Candidates[0]
	if candidate.Classification != ClassificationReview {
		t.Fatalf("classification = %s reasons=%+v", candidate.Classification, candidate.Reasons)
	}
	if candidate.Target == nil || candidate.Target.ID != "v2" || candidate.Target.SHA256 != digest {
		t.Fatalf("target = %+v", candidate.Target)
	}
	if candidate.ProjectURL != "https://github.com/example/mod" {
		t.Fatalf("project URL = %q", candidate.ProjectURL)
	}
	if len(candidate.Changelogs) != 1 || candidate.Changelogs[0].Body != "New feature" {
		t.Fatalf("changelogs = %+v", candidate.Changelogs)
	}
}
