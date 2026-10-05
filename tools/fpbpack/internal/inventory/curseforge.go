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
	DefaultCurseForgeAPI = "https://api.curseforge.com"
	curseForgeBatchSize  = 100
)

type CurseForgeClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	UserAgent  string
}

type curseForgeFile struct {
	ID              uint32   `json:"id"`
	ModID           uint32   `json:"modId"`
	FileFingerprint uint32   `json:"fileFingerprint"`
	IsAvailable     bool     `json:"isAvailable"`
	DisplayName  string   `json:"displayName"`
	FileName     string   `json:"fileName"`
	ReleaseType  int      `json:"releaseType"`
	GameVersions []string `json:"gameVersions"`
}

type curseForgeExactMatch struct {
	ID   uint32         `json:"id"`
	File curseForgeFile `json:"file"`
}

type curseForgeFingerprintResponse struct {
	Data struct {
		ExactMatches []curseForgeExactMatch `json:"exactMatches"`
	} `json:"data"`
}

func (client CurseForgeClient) Match(ctx context.Context, mods []ModFile) (map[uint32]CurseForgeMatch, error) {
	if strings.TrimSpace(client.APIKey) == "" {
		return nil, fmt.Errorf("CurseForge API key is required")
	}
	if client.BaseURL == "" {
		client.BaseURL = DefaultCurseForgeAPI
	}
	if client.HTTPClient == nil {
		client.HTTPClient = http.DefaultClient
	}
	if client.UserAgent == "" {
		client.UserAgent = "fpbcraft/fpbpack (https://github.com/fpbcraft/fpbcraft)"
	}

	unique := make([]uint32, 0, len(mods))
	seen := make(map[uint32]struct{}, len(mods))
	for _, mod := range mods {
		if mod.Modrinth != nil || mod.CurseForgeFingerprint == 0 {
			continue
		}
		if _, ok := seen[mod.CurseForgeFingerprint]; ok {
			continue
		}
		seen[mod.CurseForgeFingerprint] = struct{}{}
		unique = append(unique, mod.CurseForgeFingerprint)
	}

	matches := make(map[uint32]CurseForgeMatch)
	for start := 0; start < len(unique); start += curseForgeBatchSize {
		end := start + curseForgeBatchSize
		if end > len(unique) {
			end = len(unique)
		}
		batch, err := client.matchBatch(ctx, unique[start:end])
		if err != nil {
			return nil, err
		}
		for fingerprint, match := range batch {
			matches[fingerprint] = match
		}
	}
	return matches, nil
}

func (client CurseForgeClient) matchBatch(ctx context.Context, fingerprints []uint32) (map[uint32]CurseForgeMatch, error) {
	body, err := json.Marshal(map[string]any{"fingerprints": fingerprints})
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(client.BaseURL, "/") + "/v1/fingerprints/432"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-api-key", client.APIKey)
	request.Header.Set("User-Agent", client.UserAgent)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("CurseForge lookup: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("CurseForge lookup returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}

	var decoded curseForgeFingerprintResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 16<<20))
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode CurseForge response: %w", err)
	}

	matches := make(map[uint32]CurseForgeMatch, len(decoded.Data.ExactMatches))
	for _, exact := range decoded.Data.ExactMatches {
		fingerprint := exact.File.FileFingerprint
		if fingerprint == 0 || exact.File.ID == 0 || exact.File.ModID == 0 || !exact.File.IsAvailable {
			continue
		}
		matches[fingerprint] = CurseForgeMatch{
			ProjectID:    exact.File.ModID,
			FileID:       exact.File.ID,
			DisplayName:  exact.File.DisplayName,
			Filename:     exact.File.FileName,
			GameVersions: exact.File.GameVersions,
			ReleaseType:  exact.File.ReleaseType,
		}
	}
	return matches, nil
}

func ApplyCurseForgeMatches(result *Inventory, matches map[uint32]CurseForgeMatch) {
	for index := range result.Mods {
		if result.Mods[index].Modrinth != nil {
			continue
		}
		if match, ok := matches[result.Mods[index].CurseForgeFingerprint]; ok {
			copy := match
			result.Mods[index].CurseForge = &copy
		}
	}
	result.CurseForgeChecked = true
	result.CurseForgeError = ""
	result.RecalculateSummary()
}
