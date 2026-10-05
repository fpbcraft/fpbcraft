package updates

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

type Options struct {
	Minecraft        string
	Loader           string
	ModrinthBaseURL  string
	CurseForgeBaseURL string
	CurseForgeAPIKey string
	GitHubBaseURL     string
	GitHubToken       string
	HTTPClient       *http.Client
	Mode             RefreshMode
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
	installedCurseForge := map[string]catalog.Entry{}
	projectIDs := make([]string, 0)
	for _, entry := range cat.Managed {
		switch entry.Provider {
		case "modrinth":
			installedModrinth[entry.ProjectID] = entry
			projectIDs = append(projectIDs, entry.ProjectID)
		case "curseforge":
			installedCurseForge[entry.ProjectID] = entry
		}
	}

	client := &ModrinthClient{BaseURL: opts.ModrinthBaseURL, Mode: opts.Mode}
	if opts.HTTPClient != nil {
		client.HTTPClient = opts.HTTPClient
	}
	projects, projectErr := client.ListProjects(ctx, projectIDs)
	curseForgeClient := &CurseForgeClient{
		BaseURL: opts.CurseForgeBaseURL,
		APIKey: opts.CurseForgeAPIKey,
		HTTPClient: opts.HTTPClient,
		Mode: opts.Mode,
	}
	gitHubClient := &GitHubClient{
		BaseURL: opts.GitHubBaseURL,
		Token: opts.GitHubToken,
		HTTPClient: opts.HTTPClient,
		Mode: opts.Mode,
	}

	candidates := make([]Candidate, len(cat.Managed))
	semaphore := make(chan struct{}, providerConcurrency(opts.Mode))
	var wait sync.WaitGroup

	for index, entry := range cat.Managed {
		index, entry := index, entry
		wait.Add(1)
		go func() {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				candidates[index] = blockedProviderCandidate(
					entry,
					"provider_discovery_cancelled",
					"Update discovery was cancelled before this provider could be checked.",
				)
				return
			}

			switch entry.Provider {
			case "modrinth":
				candidate := discoverModrinthCandidate(ctx, client, entry, projects[entry.ProjectID], installedModrinth, opts)
				if projectErr != nil {
					candidate.Reasons = append(candidate.Reasons, Reason{
						Code: "project_metadata_unavailable",
						Message: "Project metadata could not be refreshed; installed identity was retained.",
					})
				}
				candidates[index] = candidate
			case "curseforge":
				if strings.TrimSpace(opts.CurseForgeAPIKey) == "" {
					candidates[index] = blockedProviderCandidate(
						entry,
						"curseforge_api_key_missing",
						"CurseForge update discovery requires an API key configured in Settings → Providers or FPBPACK_CURSEFORGE_API_KEY.",
					)
				} else {
					candidates[index] = discoverCurseForgeCandidate(
						ctx,
						curseForgeClient,
						entry,
						installedCurseForge,
						opts,
					)
				}
			case "github":
				if strings.TrimSpace(entry.Repository) == "" && strings.TrimSpace(entry.ProjectID) == "" {
					candidates[index] = blockedProviderCandidate(
						entry,
						"github_source_incomplete",
						"GitHub release discovery requires a verified repository source.",
					)
				} else {
					candidates[index] = discoverGitHubCandidate(ctx, gitHubClient, entry)
				}
			default:
				candidates[index] = blockedProviderCandidate(
					entry,
					"unsupported_provider",
					"No update provider is registered for this managed artifact.",
				)
			}
		}()
	}
	wait.Wait()
	report.Candidates = append(report.Candidates, candidates...)

	populateReverseDependencies(report.Candidates)
	for index := range report.Candidates {
		report.Candidates[index].BaseClassification = report.Candidates[index].Classification
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

func populateReverseDependencies(candidates []Candidate) {
	byKey := make(map[string]int, len(candidates))
	for index, candidate := range candidates {
		byKey[candidate.Provider+":"+candidate.ProjectID] = index
	}
	for _, candidate := range candidates {
		appendRequiredBy(candidates, byKey, candidate.Name, candidate.Dependencies)
	}
	for index := range candidates {
		candidates[index].RequiredBy = normalizeNames(candidates[index].RequiredBy)
	}
}

func appendRequiredBy(candidates []Candidate, byKey map[string]int, owner string, dependencies []Dependency) {
	for _, dependency := range dependencies {
		if dependency.Type != "required" {
			continue
		}
		if index, ok := byKey[dependency.Provider+":"+dependency.ProjectID]; ok && owner != "" {
			candidates[index].RequiredBy = append(candidates[index].RequiredBy, owner)
		}
		nextOwner := dependency.Name
		if nextOwner == "" {
			nextOwner = dependency.ProjectID
		}
		appendRequiredBy(candidates, byKey, nextOwner, dependency.Dependencies)
	}
}

func normalizeNames(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func blockedProviderCandidate(entry catalog.Entry, code, message string) Candidate {
	candidate := Candidate{
		Key:            catalog.EntryKey(entry),
		Provider:       entry.Provider,
		ProjectID:      entry.ProjectID,
		Name:           entry.Name,
		Side:           entry.Side,
		Deployment:     entry.Deployment,
		Installed:      installedRelease(entry),
		Classification: ClassificationBlocked,
		Reasons:        []Reason{{Code: code, Message: message}},
	}
	switch entry.Provider {
	case "github":
		if entry.Repository != "" {
			candidate.ProjectURL = "https://github.com/" + entry.Repository
		}
	}
	return candidate
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
		Key:        catalog.EntryKey(entry),
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

	// Keep the broad version-history request lightweight so large projects do
	// not need to return years of changelog text. Hydrate changelogs only for
	// versions compatible with the configured Minecraft/loader pair.
	changelogVersions, changelogErr := client.ListCompatibleVersionsWithChangelog(
		ctx,
		entry.ProjectID,
		opts.Minecraft,
		opts.Loader,
	)
	if changelogErr == nil {
		relevant := make([]modrinthVersion, 0, len(changelogVersions))
		for _, version := range changelogVersions {
			if !version.DatePublished.After(current.DatePublished) {
				continue
			}
			if len(rejectionReasons(version, opts.Minecraft, opts.Loader)) > 0 {
				continue
			}
			relevant = append(relevant, version)
		}
		candidate.Changelogs = changelogEntries(relevant)
	} else {
		candidate.Changelogs = changelogEntries(valid)
		candidate.Reasons = append(candidate.Reasons, Reason{
			Code: "changelog_refresh_failed",
			Message: "Compatible releases were resolved, but their changelogs could not be refreshed: " + changelogErr.Error(),
		})
	}

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

	resolver := dependencyResolver{
		ctx:       ctx,
		client:    client,
		installed: installed,
		opts:      opts,
	}
	for _, dependency := range target.Dependencies {
		candidate.Dependencies = append(
			candidate.Dependencies,
			resolver.resolve(dependency, entry.Deployment, &candidate, map[string]bool{}),
		)
	}
	return candidate
}

func changelogEntries(versions []modrinthVersion) []ChangelogEntry {
	if len(versions) == 0 {
		return nil
	}
	ordered := append([]modrinthVersion(nil), versions...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].DatePublished.Before(ordered[j].DatePublished)
	})
	result := make([]ChangelogEntry, 0, len(ordered))
	for _, version := range ordered {
		result = append(result, ChangelogEntry{
			ID:          version.ID,
			Number:      version.VersionNumber,
			Name:        version.Name,
			PublishedAt: version.DatePublished,
			Channel:     version.VersionType,
			Body:        strings.TrimSpace(version.Changelog),
		})
	}
	return result
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

type dependencyResolver struct {
	ctx       context.Context
	client    *ModrinthClient
	installed map[string]catalog.Entry
	opts      Options
}

func (r dependencyResolver) resolve(
	dep modrinthDependency,
	parentDeployment inventory.Location,
	candidate *Candidate,
	visiting map[string]bool,
) Dependency {
	result := Dependency{
		Provider:  "modrinth",
		ProjectID: dep.ProjectID,
		VersionID: dep.VersionID,
		Type:      dep.DependencyType,
		Action:    "none",
	}

	var exact *modrinthVersion
	if result.ProjectID == "" && dep.VersionID != "" {
		version, err := r.client.GetVersion(r.ctx, dep.VersionID)
		if err != nil {
			if dep.DependencyType == "required" {
				result.Action = "unresolved"
				promote(candidate, ClassificationBlocked, Reason{
					Code: "required_dependency_lookup_failed",
					Message: "A required dependency version could not be resolved: " + err.Error(),
				})
			}
			return result
		}
		exact = &version
		result.ProjectID = version.ProjectID
	}
	if result.ProjectID == "" {
		if dep.DependencyType == "required" {
			result.Action = "unresolved"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "unresolved_required_dependency",
				Message: "A required dependency does not identify a project and cannot be resolved automatically.",
			})
		}
		return result
	}

	installedEntry, exists := r.installed[result.ProjectID]
	if exists {
		result.InstalledVersion = installedEntry.VersionID
		result.Name = installedEntry.Name
		result.Deployment = installedEntry.Deployment
	}
	if result.Name == "" {
		result.Name = result.ProjectID
	}

	switch dep.DependencyType {
	case "required":
		if dep.VersionID == "" && exists {
			result.Action = "satisfied"
			return result
		}

		target, err := r.resolveTarget(dep, result.ProjectID, exact)
		if err != nil {
			result.Action = "unresolved"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "required_dependency_unresolved",
				Message: err.Error(),
			})
			return result
		}
		if target.ProjectID != result.ProjectID {
			result.Action = "unresolved"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "required_dependency_project_mismatch",
				Message: "A required dependency version belongs to a different Modrinth project.",
			})
			return result
		}
		if rejected := rejectionReasons(target, r.opts.Minecraft, r.opts.Loader); len(rejected) > 0 {
			result.Action = "unresolved"
			promote(candidate, ClassificationBlocked, Reason{
				Code: "required_dependency_incompatible",
				Message: "A required dependency target is not compatible with the configured Minecraft version and loader.",
			})
			return result
		}

		release := releaseFromModrinth(target)
		result.Target = &release
		result.TargetVersion = target.ID
		if result.Deployment == "" {
			result.Deployment = dependencyDeployment(target.Environment, parentDeployment)
		}

		if exists && installedEntry.VersionID == target.ID {
			result.Action = "satisfied"
			return result
		}
		if exists {
			result.Action = "update"
			promote(candidate, ClassificationReview, Reason{
				Code: "required_dependency_update",
				Message: "The target release requires an update to an installed dependency.",
			})
		} else {
			result.Action = "add"
			promote(candidate, ClassificationReview, Reason{
				Code: "required_dependency_addition",
				Message: "The target release requires an additional Modrinth project.",
			})
		}

		if target.VersionType != "" && target.VersionType != "release" {
			promote(candidate, ClassificationReview, Reason{
				Code: "required_dependency_prerelease",
				Message: "A required dependency resolves to a " + target.VersionType + " release.",
			})
		}

		visitKey := result.ProjectID + ":" + target.ID
		if visiting[visitKey] {
			return result
		}
		visiting[visitKey] = true
		for _, nested := range target.Dependencies {
			result.Dependencies = append(
				result.Dependencies,
				r.resolve(nested, result.Deployment, candidate, visiting),
			)
		}
		delete(visiting, visitKey)

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

func (r dependencyResolver) resolveTarget(
	dep modrinthDependency,
	projectID string,
	exact *modrinthVersion,
) (modrinthVersion, error) {
	if exact != nil {
		return *exact, nil
	}
	if dep.VersionID != "" {
		version, err := r.client.GetVersion(r.ctx, dep.VersionID)
		if err != nil {
			return modrinthVersion{}, fmt.Errorf("required dependency %s version %s could not be loaded: %w", projectID, dep.VersionID, err)
		}
		return version, nil
	}

	versions, err := r.client.ListCompatibleVersions(r.ctx, projectID, r.opts.Minecraft, r.opts.Loader)
	if err != nil {
		return modrinthVersion{}, fmt.Errorf("required dependency %s versions could not be loaded: %w", projectID, err)
	}
	compatible := make([]modrinthVersion, 0, len(versions))
	for _, version := range versions {
		if len(rejectionReasons(version, r.opts.Minecraft, r.opts.Loader)) == 0 {
			compatible = append(compatible, version)
		}
	}
	if len(compatible) == 0 {
		return modrinthVersion{}, fmt.Errorf(
			"required dependency %s has no listed version compatible with Minecraft %s and %s",
			projectID,
			r.opts.Minecraft,
			r.opts.Loader,
		)
	}
	sort.Slice(compatible, func(i, j int) bool {
		return compatible[i].DatePublished.After(compatible[j].DatePublished)
	})
	for _, version := range compatible {
		if version.VersionType == "" || version.VersionType == "release" {
			return version, nil
		}
	}
	return compatible[0], nil
}

func dependencyDeployment(environment string, fallback inventory.Location) inventory.Location {
	switch environment {
	case "client_only", "singleplayer_only":
		return inventory.LocationClient
	case "server_only", "dedicated_server_only":
		return inventory.LocationServer
	default:
		return fallback
	}
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
	id := entry.VersionID
	if entry.Provider == "curseforge" && entry.FileID != 0 {
		id = strconv.FormatUint(uint64(entry.FileID), 10)
	}
	return Release{
		ID: id,
		Name: entry.Name,
		Filename: entry.Filename,
		URL: entry.URL,
		SHA1: entry.SHA1,
		SHA512: entry.SHA512,
	}
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
