package service

import (
	"testing"
	"time"

	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func TestPreserveFailedMetadataKeepsLastGoodCandidate(t *testing.T) {
	now := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	previous := updatecheck.Report{
		GeneratedAt: now,
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:test",
			Provider: "modrinth",
			ProjectID: "test",
			Name: "Test",
			ProjectURL: "https://modrinth.com/mod/test",
			IconURL: "https://cdn.example/icon.png",
			Classification: updatecheck.ClassificationSafe,
			BaseClassification: updatecheck.ClassificationSafe,
			Target: &updatecheck.Release{ID: "v2", Number: "2.0"},
			Changelogs: []updatecheck.ChangelogEntry{{ID: "v2", Body: "last good changelog"}},
		}},
	}
	fresh := updatecheck.Report{
		GeneratedAt: now.Add(time.Hour),
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:test",
			Provider: "modrinth",
			ProjectID: "test",
			Name: "Test",
			Classification: updatecheck.ClassificationBlocked,
			Reasons: []updatecheck.Reason{{
				Code: "provider_lookup_failed",
				Message: "HTTP 503",
			}},
		}},
	}

	preserveFailedMetadata(previous, &fresh)
	got := fresh.Candidates[0]
	if !got.MetadataStale {
		t.Fatal("expected preserved candidate to be marked stale")
	}
	if got.RefreshError != "HTTP 503" {
		t.Fatalf("refresh error = %q", got.RefreshError)
	}
	if got.Target == nil || got.Target.ID != "v2" {
		t.Fatalf("target metadata was flushed: %+v", got.Target)
	}
	if len(got.Changelogs) != 1 || got.Changelogs[0].Body != "last good changelog" {
		t.Fatalf("changelog metadata was flushed: %+v", got.Changelogs)
	}
	if got.Classification != updatecheck.ClassificationSafe {
		t.Fatalf("classification = %s, want preserved safe", got.Classification)
	}
}

func TestPreserveFailedMetadataClearsStaleOnSuccessfulRefresh(t *testing.T) {
	previous := updatecheck.Report{
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:test",
			Provider: "modrinth",
			ProjectID: "test",
			MetadataStale: true,
			RefreshError: "old failure",
		}},
	}
	fresh := updatecheck.Report{
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:test",
			Provider: "modrinth",
			ProjectID: "test",
			Classification: updatecheck.ClassificationUpToDate,
		}},
	}
	preserveFailedMetadata(previous, &fresh)
	if fresh.Candidates[0].MetadataStale || fresh.Candidates[0].RefreshError != "" {
		t.Fatalf("successful refresh stayed stale: %+v", fresh.Candidates[0])
	}
}
