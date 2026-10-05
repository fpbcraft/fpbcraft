package updates

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

type Options struct {
	Minecraft       string
	Loader          string
	ModrinthBaseURL string
	HTTPClient      *http.Client
}

func Discover(ctx context.Context, cat catalog.Report, opts Options) Report {
	if strings.TrimSpace(opts.Minecraft) == "" {
		opts.Minecraft = "1.21.1"
	}
	if strings.TrimSpace(opts.Loader) == "" {
		opts.Loader = "neoforge"
	}

	report := Report{
		GeneratedAt: time.Now().UTC(),
		Minecraft:   opts.Minecraft,
		Loader:      opts.Loader,
	}

	installedModrinth := map[string]catalog.Entry{}
	projectIDs := make([]string, 0)
	for _, entry := range cat.Managed {
		if entry.Provider == "modrinth" {
			installedModrinth[entry.ProjectID] = entry
			projectIDs = append(projectIDs, entry.ProjectID)
		}
	}

	client := &ModrinthClient{BaseURL: opts.ModrinthBaseURL}
	if opts.HTTPClient != nil {
		client.HTTPClient = opts.HTTPClient
	}
	projects, projectErr := client.ListProjects(ctx, projectIDs)

	for _, entry := range cat.Managed {
		switch entry.Provider {
		case "modrinth":
			candidate := discoverModrinthCandidate(ctx, client, entry, projects[entry.ProjectID], installedModrinth, opts)
			if projectErr != nil {
				candidate.Reasons = append(candidate.Reasons, Reason{
					Code: "project_metadata_unavailable",
					Message: "Project metadata could not be refreshed; installed identity was retained.",
				})
			}
			report.Candidates = append(report.Candidates, candidate)
		case "curseforge", "github":
			report.Candidates = append(report.Candidates, Candidate{
				Key:            entry.Provider + ":" + entry.ProjectID,
				Provider:       entry.Provider,
				ProjectID:      entry.ProjectID,
				Name:           entry.Name,
				Side:           entry.Side,
				Deployment:     entry.Deployment,
				Installed:      installedRelease(entry),
				Classification: ClassificationBlocked,
				Reasons: []Reason{{
					Code: "provider_discovery_pending",
					Message: "Update discovery for this provider is not implemented yet.",
				}},
			})
		default:
			report.Candidates = append(report.Candidates, Candidate{
				Key:            entry.Provider + ":" + entry.ProjectID,
				Provider:       entry.Provider,
				ProjectID:      entry.ProjectID,
				Name:           entry.Name,
				Side:           entry.Side,
				Deployment:     entry.Deployment,
				Installed:      installedRelease(entry),
				Classification: ClassificationBlocked,
				Reasons: []Reason{{Code: "unsupported_provider", Message: "No update provider is registered for this managed artifact."}},
			})
		}
	}

	sort.Slice(report.Candidates, func(i, j int) bool {
		if report.Candidates[i].Name != report.Candidates[j].Name {
			return strings.ToLower(report.Candidates[i].Name) < strings.ToLower(report.Candidates[j].Name)
		}
		return report.Candidates[i].Key < report.Candidates[j].Key
	})
	report.RecalculateSummary()
	return report
}

func discoverModrinthCandidate(
	ctx context.Context,
	client *ModrinthClient,
	entry catalog.Entry,
	project modrinthProject,
	installed map[string]catalog.Entry,
	opts Options,
) Candidate {
	name := entry.Name
	projectURL := "https://modrinth.com/mod/" + entry.ProjectID
	if project.Title != "" {
		name = project.Title
	}
	if project.Slug != "" {
		projectURL = "https://modrinth.com/mod/" + project.Slug
	}
	candidate := Candidate{
		Key:        "modrinth:" + entry.ProjectID,
		Provider:   "modrinth",
		ProjectID:  entry.ProjectID,
		Name:       name,
		ProjectURL: projectURL,
		IconURL:    project.IconURL,
		Side:       entry.Side,
		Deployment: entry.Deployment,
	}

	versions, err := client.ListVersions(ctx, entry.ProjectID)
	if err != nil {
		candidate.Installed = installedRelease(entry)
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{Code: "provider_lookup_failed", Message: err.Error()}}
		return candidate
	}

	var current *modrinthVersion
	for index := range versions {
		if versions[index].ID == entry.VersionID {
			copy := versions[index]
			current = &copy
			break
		}
	}
	if current == nil {
		candidate.Installed = installedRelease(entry)
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{
			Code: "installed_version_not_returned",
			Message: "The installed Modrinth version was not returned by the project version list, so newer-version ordering cannot be proven safely.",
		}}
		return candidate
	}
	candidate.Installed = releaseFromModrinth(*current)

	valid := make([]modrinthVersion, 0)
	for _, version := range versions {
		if !version.DatePublished.After(current.DatePublished) {
			continue
		}
		rejected := rejectionReasons(version, opts.Minecraft, opts.Loader)
		if len(rejected) > 0 {
			candidate.Rejected = append(candidate.Rejected, RejectedVersion{
				ID: version.ID, Number: version.VersionNumber, Reasons: rejected,
			})
			continue
		}
		valid = append(valid, version)
	}

	if len(valid) == 0 {
		candidate.Classification = ClassificationUpToDate
		return candidate
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].DatePublished.After(valid[j].DatePublished) })
	target := valid[0]
	release := releaseFromModrinth(target)
	candidate.Target = &release
	candidate.Classification = ClassificationSafe

	if target.VersionType != "" && target.VersionType != "release" {
		promote(&candidate, ClassificationReview, Reason{
			Code: "prerelease",
			Message: "The newest compatible version is a " + target.VersionType + " release.",
		})
	}
	currentMajor, currentOK := versionMajor(current.VersionNumber)
	targetMajor, targetOK := versionMajor(target.VersionNumber)
	if currentOK && targetOK && currentMajor != targetMajor {
		promote(&candidate, ClassificationReview, Reason{
			Code: "major_version_jump",
			Message: fmt.Sprintf("Version changes major line from %d to %d.", currentMajor, targetMajor),
		})
	}
	if strictEnvironmentMismatch(entry.Deployment, target.Environment) {
		promote(&candidate, ClassificationBlocked, Reason{
			Code: "environment_mismatch",
			Message: "The target release environment is incompatible with the current deployment location.",
		})
	}

	for _, dependency := range target.Dependencies {
		candidate.Dependencies = append(candidate.Dependencies, classifyDependency(dependency, installed, &candidate))
	}
	return candidate
}

func rejectionReasons(version modrinthVersion, minecraft, loader string) []Reason {
	reasons := make([]Reason, 0, 3)
	if version.Status != "" && version.Status != "listed" {
		reasons = append(reasons, Reason{Code: "not_listed", Message: "Version status is " + version.Status + "."})
	}
	if !containsFold(version.GameVersions, minecraft) {
		reasons = append(reasons, Reason{Code: "minecraft_mismatch", Message: "Version does not support Minecraft " + minecraft + "."})
	}
	if !containsFold(version.Loaders, loader) {
		reasons = append(reasons, Reason{Code: "loader_mismatch", Message: "Version does not support " + loader + "."})
	}
	return reasons
}

func classifyDependency(dep modrinthDependency, installed map[string]catalog.Entry, candidate *Candidate) Dependency {
	result := Dependency{
		Provider:  "modrinth",
		ProjectID: dep.ProjectID,
		VersionID: dep.VersionID,
		Type:      dep.DependencyType,
		Action:    "none",
	}
	if dep.ProjectID == "" {
		if dep.DependencyType == "required" {
			result.Action = "unresolved"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "unresolved_required_dependency",
				Message: "A required dependency does not identify a project and cannot be resolved automatically.",
			})
		}
		return result
	}

	installedEntry, exists := installed[dep.ProjectID]
	if exists {
		result.InstalledVersion = installedEntry.VersionID
	}

	switch dep.DependencyType {
	case "required":
		if !exists {
			result.Action = "add"
			result.TargetVersion = dep.VersionID
			promote(candidate, ClassificationReview, Reason{
				Code: "required_dependency_addition",
				Message: "The target release requires an additional Modrinth project.",
			})
		} else if dep.VersionID != "" && installedEntry.VersionID != dep.VersionID {
			result.Action = "update"
			result.TargetVersion = dep.VersionID
			promote(candidate, ClassificationReview, Reason{
				Code: "required_dependency_update",
				Message: "The target release requires a different version of an installed dependency.",
			})
		} else {
			result.Action = "satisfied"
		}
	case "incompatible":
		if exists {
			result.Action = "conflict"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "incompatible_dependency_installed",
				Message: "The target release declares an installed Modrinth project incompatible.",
			})
		}
	case "optional":
		result.Action = "optional"
	case "embedded":
		result.Action = "embedded"
	}
	return result
}

func releaseFromModrinth(version modrinthVersion) Release {
	release := Release{
		ID: version.ID, Number: version.VersionNumber, Name: version.Name,
		PublishedAt: version.DatePublished, Channel: version.VersionType,
	}
	if len(version.Files) == 0 {
		return release
	}
	file := version.Files[0]
	for _, candidate := range version.Files {
		if candidate.Primary {
			file = candidate
			break
		}
	}
	release.Filename = file.Filename
	release.URL = file.URL
	release.SHA512 = file.Hashes.SHA512
	return release
}

func installedRelease(entry catalog.Entry) Release {
	return Release{ID: entry.VersionID, Name: entry.Name, Filename: entry.Filename, URL: entry.URL, SHA512: entry.SHA512}
}

func strictEnvironmentMismatch(deployment inventory.Location, environment string) bool {
	if deployment == inventory.LocationClient {
		return environment == "server_only" || environment == "dedicated_server_only"
	}
	return environment == "client_only" || environment == "singleplayer_only"
}

func promote(candidate *Candidate, classification Classification, reason Reason) {
	if classRank(classification) > classRank(candidate.Classification) {
		candidate.Classification = classification
	}
	candidate.Reasons = append(candidate.Reasons, reason)
}

func classRank(classification Classification) int {
	switch classification {
	case ClassificationBlocked:
		return 3
	case ClassificationReview:
		return 2
	case ClassificationSafe:
		return 1
	default:
		return 0
	}
}

func containsFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(value, expected) {
			return true
		}
	}
	return false
}

func versionMajor(version string) (int, bool) {
	version = strings.TrimSpace(version)
	start := -1
	for index, r := range version {
		if r >= '0' && r <= '9' {
			start = index
			break
		}
	}
	if start < 0 {
		return 0, false
	}
	end := start
	for end < len(version) && version[end] >= '0' && version[end] <= '9' {
		end++
	}
	major, err := strconv.Atoi(version[start:end])
	return major, err == nil
}
