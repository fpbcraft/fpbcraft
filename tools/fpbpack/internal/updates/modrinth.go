package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultModrinthAPI = "https://api.modrinth.com/v2"

type ModrinthClient struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
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

func (client *ModrinthClient) ListVersions(ctx context.Context, projectID string) ([]modrinthVersion, error) {
	endpoint := client.baseURL() + "/project/" + url.PathEscape(projectID) + "/version?include_changelog=false"
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
	const maxAttempts = 4
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", client.userAgent())

		resp, err := client.httpClient().Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			wait := retryAfter(resp.Header.Get("Retry-After"))
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			if attempt == maxAttempts {
				return fmt.Errorf("GET %s returned HTTP 429 after %d attempts", endpoint, attempt)
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			return fmt.Errorf("GET %s returned HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(message)))
		}
		decoder := json.NewDecoder(io.LimitReader(resp.Body, 32<<20))
		err = decoder.Decode(target)
		closeErr := resp.Body.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	return fmt.Errorf("GET %s failed", endpoint)
}

func retryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 2 * time.Second
	}
	seconds, err := strconv.Atoi(value)
	if err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err == nil {
		wait := time.Until(when)
		if wait > 0 {
			return wait
		}
	}
	return 2 * time.Second
}
