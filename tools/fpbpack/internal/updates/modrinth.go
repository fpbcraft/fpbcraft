package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultModrinthAPI = "https://api.modrinth.com/v2"

type ModrinthClient struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
	Mode       RefreshMode
}

type modrinthProject struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Slug    string `json:"slug"`
	IconURL string `json:"icon_url"`
}

type modrinthDependency struct {
	VersionID      string `json:"version_id"`
	ProjectID      string `json:"project_id"`
	FileName       string `json:"file_name"`
	DependencyType string `json:"dependency_type"`
}

type modrinthVersion struct {
	ID            string                `json:"id"`
	ProjectID     string                `json:"project_id"`
	Name          string                `json:"name"`
	VersionNumber string                `json:"version_number"`
	VersionType   string                `json:"version_type"`
	Status        string                `json:"status"`
	DatePublished time.Time             `json:"date_published"`
	Changelog     string                `json:"changelog"`
	GameVersions  []string              `json:"game_versions"`
	Loaders       []string              `json:"loaders"`
	Environment   string                `json:"environment"`
	Dependencies  []modrinthDependency  `json:"dependencies"`
	Files         []struct {
		Hashes struct {
			SHA512 string `json:"sha512"`
		} `json:"hashes"`
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Primary  bool   `json:"primary"`
	} `json:"files"`
}

type VerifiedModrinthSource struct {
	ProjectID     string
	VersionID     string
	VersionNumber string
	VersionName   string
	Filename      string
	DownloadURL   string
	Environment   string
}

func (client *ModrinthClient) VerifyInstalledVersion(
	ctx context.Context,
	projectID string,
	versionID string,
	filename string,
	expectedSHA512 string,
) (VerifiedModrinthSource, error) {
	projectID = strings.TrimSpace(projectID)
	versionID = strings.TrimSpace(versionID)
	filename = strings.TrimSpace(filename)
	expectedSHA512 = strings.ToLower(strings.TrimSpace(expectedSHA512))
	if projectID == "" || versionID == "" || expectedSHA512 == "" {
		return VerifiedModrinthSource{}, fmt.Errorf("project ID, version ID, and current SHA-512 are required")
	}

	version, err := client.GetVersion(ctx, versionID)
	if err != nil {
		return VerifiedModrinthSource{}, err
	}
	if version.ProjectID != projectID {
		return VerifiedModrinthSource{}, fmt.Errorf("Modrinth version %s belongs to project %s, not %s", versionID, version.ProjectID, projectID)
	}

	var matched *struct {
		Hashes struct {
			SHA512 string `json:"sha512"`
		} `json:"hashes"`
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Primary  bool   `json:"primary"`
	}
	for index := range version.Files {
		file := &version.Files[index]
		if filename != "" && file.Filename != filename {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(file.Hashes.SHA512), expectedSHA512) {
			matched = file
			break
		}
	}
	if matched == nil && filename == "" {
		for index := range version.Files {
			file := &version.Files[index]
			if strings.EqualFold(strings.TrimSpace(file.Hashes.SHA512), expectedSHA512) {
				matched = file
				break
			}
		}
	}
	if matched == nil {
		return VerifiedModrinthSource{}, fmt.Errorf("Modrinth version %s does not contain an artifact matching the installed JAR SHA-512", versionID)
	}

	return VerifiedModrinthSource{
		ProjectID: projectID,
		VersionID: version.ID,
		VersionNumber: version.VersionNumber,
		VersionName: version.Name,
		Filename: matched.Filename,
		DownloadURL: matched.URL,
		Environment: version.Environment,
	}, nil
}

func (client *ModrinthClient) ListProjects(ctx context.Context, ids []string) (map[string]modrinthProject, error) {
	result := make(map[string]modrinthProject, len(ids))
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	const batchSize = 100
	for start := 0; start < len(unique); start += batchSize {
		end := start + batchSize
		if end > len(unique) {
			end = len(unique)
		}
		encoded, err := json.Marshal(unique[start:end])
		if err != nil {
			return nil, err
		}
		endpoint := client.baseURL() + "/projects?ids=" + url.QueryEscape(string(encoded))
		var projects []modrinthProject
		if err := client.getJSON(ctx, endpoint, &projects); err != nil {
			return nil, fmt.Errorf("Modrinth projects: %w", err)
		}
		for _, project := range projects {
			result[project.ID] = project
		}
	}
	return result, nil
}

func (client *ModrinthClient) GetVersion(ctx context.Context, versionID string) (modrinthVersion, error) {
	endpoint := client.baseURL() + "/version/" + url.PathEscape(versionID)
	var version modrinthVersion
	if err := client.getJSON(ctx, endpoint, &version); err != nil {
		return modrinthVersion{}, fmt.Errorf("Modrinth version %s: %w", versionID, err)
	}
	return version, nil
}

func (client *ModrinthClient) ListVersions(ctx context.Context, projectID string) ([]modrinthVersion, error) {
	return client.listVersions(ctx, projectID, "", "")
}

func (client *ModrinthClient) ListCompatibleVersions(
	ctx context.Context,
	projectID string,
	minecraft string,
	loader string,
) ([]modrinthVersion, error) {
	return client.listVersions(ctx, projectID, minecraft, loader)
}

func (client *ModrinthClient) listVersions(
	ctx context.Context,
	projectID string,
	minecraft string,
	loader string,
) ([]modrinthVersion, error) {
	query := url.Values{}
	query.Set("include_changelog", "true")
	if minecraft = strings.TrimSpace(minecraft); minecraft != "" {
		encoded, err := json.Marshal([]string{minecraft})
		if err != nil {
			return nil, err
		}
		query.Set("game_versions", string(encoded))
	}
	if loader = strings.TrimSpace(loader); loader != "" {
		encoded, err := json.Marshal([]string{loader})
		if err != nil {
			return nil, err
		}
		query.Set("loaders", string(encoded))
	}
	endpoint := client.baseURL() + "/project/" + url.PathEscape(projectID) + "/version?" + query.Encode()
	var versions []modrinthVersion
	if err := client.getJSON(ctx, endpoint, &versions); err != nil {
		return nil, fmt.Errorf("Modrinth versions for %s: %w", projectID, err)
	}
	return versions, nil
}

func (client *ModrinthClient) baseURL() string {
	if strings.TrimSpace(client.BaseURL) == "" {
		return DefaultModrinthAPI
	}
	return strings.TrimRight(client.BaseURL, "/")
}

func (client *ModrinthClient) httpClient() *http.Client {
	if client.HTTPClient != nil {
		return client.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (client *ModrinthClient) userAgent() string {
	if strings.TrimSpace(client.UserAgent) != "" {
		return client.UserAgent
	}
	return "fpbcraft/fpbpack (https://github.com/fpbcraft/fpbcraft)"
}

func (client *ModrinthClient) getJSON(ctx context.Context, endpoint string, target any) error {
	return doJSONWithRetry(
		ctx,
		"modrinth",
		client.Mode,
		client.httpClient(),
		func() (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Accept", "application/json")
			req.Header.Set("User-Agent", client.userAgent())
			return req, nil
		},
		target,
	)
}
