package service

import "strings"

type ProviderStatus struct {
	ID                     string `json:"id"`
	Label                  string `json:"label"`
	Status                 string `json:"status"`
	Detail                 string `json:"detail"`
	CredentialConfigurable bool   `json:"credential_configurable,omitempty"`
	CredentialSource       string `json:"credential_source,omitempty"`
}

func (s *Service) ProviderStatuses() []ProviderStatus {
	return []ProviderStatus{
		s.providerStatus("modrinth"),
		s.providerStatus("curseforge"),
		s.providerStatus("github"),
	}
}

func (s *Service) providerStatus(provider string) ProviderStatus {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "modrinth":
		return ProviderStatus{
			ID:     "modrinth",
			Label:  "Modrinth",
			Status: "ready",
			Detail: "Exact file matching, updates, dependencies, project metadata, and changelogs are enabled.",
		}
	case "curseforge":
		key, source := s.effectiveCurseForgeAPIKey()
		status := ProviderStatus{
			ID:                     "curseforge",
			Label:                  "CurseForge",
			Status:                 "needs_configuration",
			Detail:                 "Add a CurseForge API key here to enable official CurseForge update discovery.",
			CredentialConfigurable: true,
		}
		if key == "" {
			return status
		}
		status.Status = "ready"
		status.CredentialSource = source
		if source == "saved" {
			status.Detail = "Official CurseForge API discovery is enabled with a key stored by FPBPack."
		} else {
			status.Detail = "Official CurseForge API discovery is enabled with FPBPACK_CURSEFORGE_API_KEY."
		}
		return status
	case "github":
		status := ProviderStatus{
			ID:     "github",
			Label:  "GitHub",
			Status: "ready",
			Detail: "Verified GitHub release sources use anonymous API access.",
		}
		if strings.TrimSpace(s.options.GitHubToken) != "" {
			status.CredentialSource = "environment"
			status.Detail = "Verified GitHub release sources use authenticated API access."
		}
		return status
	default:
		return ProviderStatus{
			ID:     provider,
			Label:  provider,
			Status: "unsupported",
			Detail: "This provider is not configured.",
		}
	}
}
