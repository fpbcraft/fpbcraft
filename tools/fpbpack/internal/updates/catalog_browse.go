package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

type modrinthSearchResponse struct {
	Hits []struct {
		ProjectID   string   `json:"project_id"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Slug        string   `json:"slug"`
		IconURL     string   `json:"icon_url"`
		Downloads   int64    `json:"downloads"`
		Environment []string `json:"environment"`
	} `json:"hits"`
}

func (client *ModrinthClient) SearchCatalogProjects(
	ctx context.Context,
	query string,
	minecraft string,
	loader string,
	limit int,
) ([]CatalogProject, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []CatalogProject{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	facets, err := json.Marshal([][]string{
		{"project_type:mod"},
		{"versions:" + strings.TrimSpace(minecraft)},
		{"categories:" + strings.ToLower(strings.TrimSpace(loader))},
	})
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("query", query)
	values.Set("facets", string(facets))
	values.Set("limit", strconv.Itoa(limit))
	values.Set("index", "relevance")

	var response modrinthSearchResponse
	if err := client.getJSON(ctx, client.baseURL()+"/search?"+values.Encode(), &response); err != nil {
		return nil, fmt.Errorf("search Modrinth projects: %w", err)
	}
	result := make([]CatalogProject, 0, len(response.Hits))
	for _, hit := range response.Hits {
		result = append(result, CatalogProject{
			Provider:    "modrinth",
			ProjectID:   hit.ProjectID,
			Name:        hit.Title,
			Slug:        hit.Slug,
			Summary:     hit.Description,
			IconURL:     hit.IconURL,
			ProjectURL:  "https://modrinth.com/mod/" + hit.Slug,
			Downloads:   hit.Downloads,
			Environment: append([]string(nil), hit.Environment...),
		})
	}
	return result, nil
}

func (client *ModrinthClient) CatalogVersions(
	ctx context.Context,
	projectID string,
	minecraft string,
	loader string,
) ([]CatalogVersion, error) {
	versions, err := client.ListCompatibleVersionsWithChangelog(ctx, projectID, minecraft, loader)
	if err != nil {
		return nil, err
	}
	compatible := make([]modrinthVersion, 0, len(versions))
	for _, version := range versions {
		if len(rejectionReasons(version, minecraft, loader)) == 0 {
			compatible = append(compatible, version)
		}
	}
	sort.Slice(compatible, func(i, j int) bool {
		return compatible[i].DatePublished.After(compatible[j].DatePublished)
	})
	result := make([]CatalogVersion, 0, len(compatible))
	for _, version := range compatible {
		release := releaseFromModrinth(version)
		result = append(result, CatalogVersion{
			ID:          release.ID,
			Number:      release.Number,
			Name:        release.Name,
			PublishedAt: release.PublishedAt,
			Channel:     release.Channel,
			Filename:    release.Filename,
			Environment: version.Environment,
			Changelog:   strings.TrimSpace(version.Changelog),
			SHA512:      release.SHA512,
		})
	}
	return result, nil
}

func (client *ModrinthClient) CatalogCandidate(
	ctx context.Context,
	projectID string,
	versionID string,
	deployment inventory.Location,
	intent string,
	current *catalog.Entry,
	cat catalog.Report,
	opts Options,
) (Candidate, error) {
	version, err := client.GetVersion(ctx, versionID)
	if err != nil {
		return Candidate{}, err
	}
	if version.ProjectID != projectID {
		return Candidate{}, fmt.Errorf("Modrinth version %s belongs to project %s, not %s", versionID, version.ProjectID, projectID)
	}
	if rejected := rejectionReasons(version, opts.Minecraft, opts.Loader); len(rejected) > 0 {
		return Candidate{}, fmt.Errorf("selected Modrinth version is not compatible with Minecraft %s and %s", opts.Minecraft, opts.Loader)
	}
	projects, err := client.ListProjects(ctx, []string{projectID})
	if err != nil {
		return Candidate{}, err
	}
	project := projects[projectID]
	name := project.Title
	if name == "" {
		name = projectID
	}
	release := releaseFromModrinth(version)
	if release.Filename == "" || release.URL == "" || release.SHA512 == "" {
		return Candidate{}, fmt.Errorf("selected Modrinth version does not expose a downloadable checksummed JAR")
	}

	key := "modrinth:" + projectID
	installedRelease := Release{}
	if current != nil {
		key = catalog.EntryKey(*current)
		installedRelease = installedReleaseFromCatalog(*current)
	}
	candidate := Candidate{
		Key:            key,
		Provider:       "modrinth",
		ProjectID:      projectID,
		Name:           name,
		ProjectURL:     modrinthProjectURL(project),
		IconURL:        project.IconURL,
		Side:           sideForDeployment(deployment),
		Deployment:     deployment,
		Environment:    version.Environment,
		Installed:      installedRelease,
		Target:         &release,
		Classification: ClassificationReview,
		Intent:         intent,
		Reasons: []Reason{{
			Code:    "explicit_version_selection",
			Message: "This exact Modrinth version was selected explicitly in Catalog Management.",
		}},
	}
	if strictEnvironmentMismatch(deployment, version.Environment) {
		promote(&candidate, ClassificationBlocked, Reason{
			Code:    "environment_mismatch",
			Message: "The selected release environment is incompatible with the chosen placement.",
		})
	}
	if version.VersionType != "" && version.VersionType != "release" {
		promote(&candidate, ClassificationReview, Reason{
			Code:    "prerelease_selected",
			Message: "The selected version is a " + version.VersionType + " release.",
		})
	}
	installed := installedEntriesForProvider(cat, "modrinth")
	resolver := dependencyResolver{
		ctx:       ctx,
		client:    client,
		installed: installed,
		opts:      opts,
	}
	for _, dependency := range version.Dependencies {
		candidate.Dependencies = append(
			candidate.Dependencies,
			resolver.resolve(dependency, deployment, &candidate, map[string]bool{}),
		)
	}
	return candidate, nil
}

type curseForgeSearchResponse struct {
	Data []curseForgeMod `json:"data"`
}

func (client *CurseForgeClient) SearchCatalogProjects(
	ctx context.Context,
	query string,
	minecraft string,
	loader string,
	limit int,
) ([]CatalogProject, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []CatalogProject{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	values := url.Values{}
	values.Set("gameId", "432")
	values.Set("classId", "6")
	values.Set("gameVersion", strings.TrimSpace(minecraft))
	values.Set("searchFilter", query)
	values.Set("pageSize", strconv.Itoa(limit))
	values.Set("sortField", "2")
	values.Set("sortOrder", "desc")
	if loaderType := curseForgeLoaderType(loader); loaderType != 0 {
		values.Set("modLoaderType", strconv.Itoa(loaderType))
	}

	var response curseForgeSearchResponse
	if err := client.getJSON(ctx, "/mods/search?"+values.Encode(), &response); err != nil {
		return nil, fmt.Errorf("search CurseForge projects: %w", err)
	}
	result := make([]CatalogProject, 0, len(response.Data))
	for _, project := range response.Data {
		projectURL := project.Links.WebsiteURL
		if projectURL == "" && project.Slug != "" {
			projectURL = "https://www.curseforge.com/minecraft/mc-mods/" + project.Slug
		}
		result = append(result, CatalogProject{
			Provider:   "curseforge",
			ProjectID:  strconv.Itoa(project.ID),
			Name:       project.Name,
			Slug:       project.Slug,
			Summary:    project.Summary,
			IconURL:    project.Logo.ThumbnailURL,
			ProjectURL: projectURL,
			Downloads:  project.DownloadCount,
		})
	}
	return result, nil
}

func (client *CurseForgeClient) CatalogVersions(
	ctx context.Context,
	projectID string,
	minecraft string,
	loader string,
) ([]CatalogVersion, error) {
	files, err := client.ListFiles(ctx, projectID, minecraft, loader)
	if err != nil {
		return nil, err
	}
	available := make([]curseForgeFile, 0, len(files))
	for _, file := range files {
		if file.IsAvailable {
			available = append(available, file)
		}
	}
	sort.Slice(available, func(i, j int) bool {
		return available[i].FileDate.After(available[j].FileDate)
	})
	result := make([]CatalogVersion, 0, len(available))
	for _, file := range available {
		result = append(result, CatalogVersion{
			ID:          strconv.Itoa(file.ID),
			Number:      file.DisplayName,
			Name:        file.DisplayName,
			PublishedAt: file.FileDate,
			Channel:     curseForgeReleaseType(file.ReleaseType),
			Filename:    file.FileName,
			SHA1:        curseForgeSHA1(file),
		})
	}
	return result, nil
}

func (client *CurseForgeClient) CatalogCandidate(
	ctx context.Context,
	projectID string,
	fileID uint32,
	deployment inventory.Location,
	intent string,
	current *catalog.Entry,
	cat catalog.Report,
	opts Options,
) (Candidate, error) {
	project, err := client.GetMod(ctx, projectID)
	if err != nil {
		return Candidate{}, err
	}
	files, err := client.ListFiles(ctx, projectID, opts.Minecraft, opts.Loader)
	if err != nil {
		return Candidate{}, err
	}
	var targetFile *curseForgeFile
	for index := range files {
		if uint32(files[index].ID) == fileID && files[index].IsAvailable {
			copy := files[index]
			targetFile = &copy
			break
		}
	}
	if targetFile == nil {
		return Candidate{}, fmt.Errorf(
			"CurseForge file %d is not an available %s/%s file for project %s",
			fileID,
			opts.Minecraft,
			opts.Loader,
			projectID,
		)
	}
	projectURL := project.Links.WebsiteURL
	if projectURL == "" && project.Slug != "" {
		projectURL = "https://www.curseforge.com/minecraft/mc-mods/" + project.Slug
	}
	downloadURL, downloadErr := client.DownloadURL(ctx, projectID, *targetFile)
	release := releaseFromCurseForge(*targetFile, downloadURL)
	if downloadErr != nil {
		if isCurseForgeManualDownload(downloadErr) {
			release.ManualDownload = true
			release.ManualURL = curseForgeManualURL(projectURL, targetFile.ID)
		} else {
			return Candidate{}, fmt.Errorf("resolve CurseForge download: %w", downloadErr)
		}
	}
	if release.SHA1 == "" {
		return Candidate{}, fmt.Errorf("selected CurseForge file has no provider SHA-1 checksum")
	}

	key := "curseforge:" + projectID
	installedRelease := Release{}
	if current != nil {
		key = catalog.EntryKey(*current)
		installedRelease = installedReleaseFromCatalog(*current)
	}
	name := project.Name
	if name == "" {
		name = projectID
	}
	candidate := Candidate{
		Key:            key,
		Provider:       "curseforge",
		ProjectID:      projectID,
		Name:           name,
		ProjectURL:     projectURL,
		IconURL:        project.Logo.ThumbnailURL,
		Side:           sideForDeployment(deployment),
		Deployment:     deployment,
		Installed:      installedRelease,
		Target:         &release,
		Classification: ClassificationReview,
		Intent:         intent,
		Reasons: []Reason{{
			Code:    "explicit_version_selection",
			Message: "This exact CurseForge file was selected explicitly in Catalog Management.",
		}},
	}
	if release.Channel != "" && release.Channel != "release" {
		promote(&candidate, ClassificationReview, Reason{
			Code:    "prerelease_selected",
			Message: "The selected file is a " + release.Channel + " release.",
		})
	}
	if release.ManualDownload {
		promote(&candidate, ClassificationReview, Reason{
			Code:    "manual_download_required",
			Message: "CurseForge requires this file to be downloaded manually before Apply.",
		})
	}
	installed := installedEntriesForProvider(cat, "curseforge")
	resolver := curseForgeDependencyResolver{
		ctx:       ctx,
		client:    client,
		installed: installed,
		opts:      opts,
	}
	for _, dependency := range targetFile.Dependencies {
		candidate.Dependencies = append(
			candidate.Dependencies,
			resolver.resolve(dependency.ModID, dependency.RelationType, deployment, &candidate, map[string]bool{}),
		)
	}
	return candidate, nil
}

func installedEntriesForProvider(cat catalog.Report, provider string) map[string]catalog.Entry {
	result := map[string]catalog.Entry{}
	for _, entry := range cat.Managed {
		if entry.Provider == provider && strings.TrimSpace(entry.ProjectID) != "" {
			result[entry.ProjectID] = entry
		}
	}
	return result
}

func installedReleaseFromCatalog(entry catalog.Entry) Release {
	id := entry.VersionID
	if entry.Provider == "curseforge" && entry.FileID != 0 {
		id = strconv.FormatUint(uint64(entry.FileID), 10)
	}
	return Release{
		ID:       id,
		Number:   id,
		Name:     entry.Name,
		Filename: entry.Filename,
		URL:      entry.URL,
		SHA1:     entry.SHA1,
		SHA512:   entry.SHA512,
	}
}

func sideForDeployment(deployment inventory.Location) string {
	if deployment == inventory.LocationClient {
		return "client"
	}
	return "both"
}

func modrinthProjectURL(project modrinthProject) string {
	if strings.TrimSpace(project.Slug) != "" {
		return "https://modrinth.com/mod/" + project.Slug
	}
	if strings.TrimSpace(project.ID) != "" {
		return "https://modrinth.com/mod/" + project.ID
	}
	return ""
}
