package service

import (
	"testing"
	"time"

	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func TestApplyRulesToReportOverridesAndRestoresClassification(t *testing.T) {
	now := time.Date(2026, 10, 5, 13, 0, 0, 0, time.UTC)
	report := updatecheck.Report{
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:test",
			Classification: updatecheck.ClassificationSafe,
			BaseClassification: updatecheck.ClassificationSafe,
			Installed: updatecheck.Release{ID: "installed"},
			Target: &updatecheck.Release{ID: "target"},
		}},
	}
	rules := map[string]UpdateRule{
		"modrinth:test": {IgnoreMod: true},
	}
	applyRulesToReport(&report, rules, now)
	if report.Candidates[0].Classification != updatecheck.ClassificationIgnored {
		t.Fatalf("classification = %s", report.Candidates[0].Classification)
	}

	applyRulesToReport(&report, map[string]UpdateRule{}, now)
	if report.Candidates[0].Classification != updatecheck.ClassificationSafe {
		t.Fatalf("classification after clear = %s", report.Candidates[0].Classification)
	}
	for _, reason := range report.Candidates[0].Reasons {
		if reason.Code == "rule_ignore_mod" {
			t.Fatalf("rule reason was not removed")
		}
	}
}

func TestApplyRulesToReportReviewLaterExpires(t *testing.T) {
	now := time.Date(2026, 10, 5, 13, 0, 0, 0, time.UTC)
	later := now.Add(24 * time.Hour)
	report := updatecheck.Report{
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:test",
			Classification: updatecheck.ClassificationReview,
			BaseClassification: updatecheck.ClassificationReview,
			Target: &updatecheck.Release{ID: "target"},
		}},
	}
	rules := map[string]UpdateRule{
		"modrinth:test": {ReviewAfter: &later},
	}
	applyRulesToReport(&report, rules, now)
	if report.Candidates[0].Classification != updatecheck.ClassificationIgnored {
		t.Fatalf("classification before review date = %s", report.Candidates[0].Classification)
	}

	applyRulesToReport(&report, rules, later.Add(time.Minute))
	if report.Candidates[0].Classification != updatecheck.ClassificationReview {
		t.Fatalf("classification after review date = %s", report.Candidates[0].Classification)
	}
}

func TestApplyRulesToReportIgnoresSpecificTargetOnly(t *testing.T) {
	report := updatecheck.Report{
		Candidates: []updatecheck.Candidate{{
			Key: "modrinth:test",
			Classification: updatecheck.ClassificationSafe,
			BaseClassification: updatecheck.ClassificationSafe,
			Target: &updatecheck.Release{ID: "target-b"},
		}},
	}
	rules := map[string]UpdateRule{
		"modrinth:test": {IgnoredVersions: []string{"target-a"}},
	}
	applyRulesToReport(&report, rules, time.Now().UTC())
	if report.Candidates[0].Classification != updatecheck.ClassificationSafe {
		t.Fatalf("different target should remain safe")
	}

	report.Candidates[0].Target.ID = "target-a"
	applyRulesToReport(&report, rules, time.Now().UTC())
	if report.Candidates[0].Classification != updatecheck.ClassificationIgnored {
		t.Fatalf("matching target should be ignored")
	}
}
