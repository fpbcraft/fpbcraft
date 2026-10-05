package service

import (
	"strings"

	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func preserveFailedMetadata(previous updatecheck.Report, fresh *updatecheck.Report) {
	if fresh == nil || len(previous.Candidates) == 0 {
		return
	}
	previousByKey := make(map[string]updatecheck.Candidate, len(previous.Candidates))
	for _, candidate := range previous.Candidates {
		previousByKey[candidate.Key] = candidate
	}

	for index := range fresh.Candidates {
		current := fresh.Candidates[index]
		old, ok := previousByKey[current.Key]
		if !ok {
			continue
		}

		if hasProjectMetadataFailure(current) {
			if current.ProjectURL == "" {
				current.ProjectURL = old.ProjectURL
			}
			if current.IconURL == "" {
				current.IconURL = old.IconURL
			}
		}

		if errMessage, failed := transientCandidateFailure(current); failed {
			preserved := old
			preserved.MetadataStale = true
			preserved.RefreshError = errMessage
			preserved.Reasons = append(
				filterRefreshFailureReasons(old.Reasons),
				updatecheck.Reason{
					Code: "metadata_refresh_failed",
					Message: "Latest provider metadata refresh failed; FPBPack is keeping the last successful metadata. " + errMessage,
				},
			)
			fresh.Candidates[index] = preserved
			continue
		}

		current.MetadataStale = false
		current.RefreshError = ""
		fresh.Candidates[index] = current
	}
	fresh.RecalculateSummary()
}

func transientCandidateFailure(candidate updatecheck.Candidate) (string, bool) {
	for _, reason := range candidate.Reasons {
		switch reason.Code {
		case "provider_lookup_failed",
			"installed_file_lookup_failed",
			"provider_discovery_cancelled":
			return reason.Message, true
		}
	}
	return "", false
}

func hasProjectMetadataFailure(candidate updatecheck.Candidate) bool {
	for _, reason := range candidate.Reasons {
		if reason.Code == "project_metadata_unavailable" {
			return true
		}
	}
	return false
}

func filterRefreshFailureReasons(reasons []updatecheck.Reason) []updatecheck.Reason {
	result := reasons[:0]
	for _, reason := range reasons {
		if reason.Code == "metadata_refresh_failed" || strings.HasPrefix(reason.Code, "provider_") {
			continue
		}
		result = append(result, reason)
	}
	return result
}
