package service

import "strings"

type ProviderStatus struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func (s *Service) ProviderStatuses() []ProviderStatus {
	curseForgeStatus := ProviderStatus{
		ID: "curseforge",
		Label: "CurseForge",
		Status: "needs_configuration",
		Detail: "Set FPBPACK_CURSEFORGE_API_KEY to enable CurseForge update discovery.",
	}
	if strings.TrimSpace(s.options.CurseForgeAPIKey) != "" {
		curseForgeStatus.Status = "ready"
		curseForgeStatus.Detail = "Official CurseForge API update discovery is enabled."
	}

	gitHubStatus := ProviderStatus{
		ID: "github",
		Label: "GitHub",
		Status: "ready",
		Detail: "Verified GitHub release sources use anonymous API access.",
	}
	if strings.TrimSpace(s.options.GitHubToken) != "" {
		gitHubStatus.Detail = "Verified GitHub release sources use authenticated API access."
	}

	return []ProviderStatus{
		{
			ID: "modrinth",
			Label: "Modrinth",
			Status: "ready",
			Detail: "Exact file matching, updates, dependencies, project metadata, and changelogs are enabled.",
		},
		curseForgeStatus,
		gitHubStatus,
	}
}
