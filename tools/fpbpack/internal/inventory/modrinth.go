package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	DefaultModrinthAPI = "https://api.modrinth.com/v2"
	modrinthBatchSize  = 100
)

type ModrinthClient struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

type modrinthVersion struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"project_id"`
	Name          string   `json:"name"`
	VersionNumber string   `json:"version_number"`
	Loaders       []string `json:"loaders"`
	GameVersions  []string `json:"game_versions"`
	Environment   string   `json:"environment"`
	Files         []struct {
		Hashes struct {
			SHA512 string `json:"sha512"`
		} `json:"hashes"`
		URL      string `json:"url"`
		Filename string `json:"filename"`
	} `json:"files"`
}

func (client ModrinthClient) Match(ctx context.Context, mods []ModFile) (map[string]ModrinthMatch, error) {
	if client.BaseURL == "" {
		client.BaseURL = DefaultModrinthAPI
	}
	if client.HTTPClient == nil {
		client.HTTPClient = http.DefaultClient
	}
	if client.UserAgent == "" {
		client.UserAgent = "fpbcraft/fpbpack (https://github.com/fpbcraft/fpbcraft)"
	}

	unique := make([]string, 0, len(mods))
	seen := make(map[string]struct{}, len(mods))
	for _, mod := range mods {
		if mod.SHA512 == "" {
			continue
		}
		if _, ok := seen[mod.SHA512]; ok {
			continue
		}
		seen[mod.SHA512] = struct{}{}
		unique = append(unique, mod.SHA512)
	}

	matches := make(map[string]ModrinthMatch)
	for start := 0; start < len(unique); start += modrinthBatchSize {
		end := start + modrinthBatchSize
		if end > len(unique) {
			end = len(unique)
		}
		batch, err := client.matchBatch(ctx, unique[start:end])
		if err != nil {
			return nil, err
		}
		for hash, match := range batch {
			matches[hash] = match
		}
	}
	return matches, nil
}

func (client ModrinthClient) matchBatch(ctx context.Context, hashes []string) (map[string]ModrinthMatch, error) {
	body, err := json.Marshal(map[string]any{
		"hashes":    hashes,
		"algorithm": "sha512",
	})
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(client.BaseURL, "/") + "/version_files"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", client.UserAgent)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Modrinth lookup: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("Modrinth lookup returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}

	versions := make(map[string]modrinthVersion)
	decoder := json.NewDecoder(io.LimitReader(response.Body, 16<<20))
	if err := decoder.Decode(&versions); err != nil {
		return nil, fmt.Errorf("decode Modrinth response: %w", err)
	}

	matches := make(map[string]ModrinthMatch, len(versions))
	for hash, version := range versions {
		match := ModrinthMatch{
			ProjectID:     version.ProjectID,
			VersionID:     version.ID,
			VersionNumber: version.VersionNumber,
			VersionName:   version.Name,
			Loaders:       version.Loaders,
			GameVersions:  version.GameVersions,
			Environment:   version.Environment,
		}
		for _, file := range version.Files {
			if file.Hashes.SHA512 == hash {
				match.Filename = file.Filename
				match.URL = file.URL
				break
			}
		}
		matches[hash] = match
	}
	return matches, nil
}

func ApplyModrinthMatches(result *Inventory, matches map[string]ModrinthMatch) {
	for index := range result.Mods {
		if match, ok := matches[result.Mods[index].SHA512]; ok {
			copy := match
			result.Mods[index].Modrinth = &copy
		}
	}
	result.ModrinthChecked = true
	result.ModrinthError = ""
	result.RecalculateSummary()
}
