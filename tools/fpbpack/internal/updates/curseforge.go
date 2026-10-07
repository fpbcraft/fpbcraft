package updates

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

const DefaultCurseForgeAPI = "https://api.curseforge.com/v1"

type CurseForgeClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Mode       RefreshMode
}

type curseForgeMod struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Summary       string `json:"summary"`
	DownloadCount int64  `json:"downloadCount"`
	Links struct {
		WebsiteURL string `json:"websiteUrl"`
	} `json:"links"`
	Logo struct {
		ThumbnailURL string `json:"thumbnailUrl"`
	} `json:"logo"`
}

type curseForgeFile struct {
	ID           int       `json:"id"`
	ModID        int       `json:"modId"`
	IsAvailable  bool      `json:"isAvailable"`
	DisplayName  string    `json:"displayName"`
	FileName     string    `json:"fileName"`
	ReleaseType  int       `json:"releaseType"`
	FileDate     time.Time `json:"fileDate"`
	FileLength      int64     `json:"fileLength"`
	FileFingerprint uint32    `json:"fileFingerprint"`
	DownloadURL     string    `json:"downloadUrl"`
	GameVersions []string  `json:"gameVersions"`
	Hashes       []struct {
		Value string `json:"value"`
		Algo  int    `json:"algo"`
	} `json:"hashes"`
	Dependencies []struct {
		ModID        int `json:"modId"`
		RelationType int `json:"relationType"`
	} `json:"dependencies"`
}

type curseForgeModResponse struct {
	Data curseForgeMod `json:"data"`
}

type curseForgeFileResponse struct {
	Data curseForgeFile `json:"data"`
}

type curseForgeFilesResponse struct {
	Data []curseForgeFile `json:"data"`
	Pagination struct {
		Index       int `json:"index"`
		PageSize    int `json:"pageSize"`
		ResultCount int `json:"resultCount"`
		TotalCount  int `json:"totalCount"`
	} `json:"pagination"`
}

type curseForgeStringResponse struct {
	Data string `json:"data"`
}

type curseForgeFingerprintResponse struct {
	Data struct {
		ExactMatches []struct {
			ID   uint32         `json:"id"`
			File curseForgeFile `json:"file"`
		} `json:"exactMatches"`
	} `json:"data"`
}

type VerifiedCurseForgeSource struct {
	ProjectID   string
	FileID      uint32
	DisplayName string
	Filename    string
}

func (client *CurseForgeClient) VerifyInstalledFile(
	ctx context.Context,
	projectID string,
	fileID uint32,
	expectedSHA1 string,
) (VerifiedCurseForgeSource, error) {
	projectID = strings.TrimSpace(projectID)
	expectedSHA1 = strings.ToLower(strings.TrimSpace(expectedSHA1))
	if projectID == "" || fileID == 0 || expectedSHA1 == "" {
		return VerifiedCurseForgeSource{}, fmt.Errorf("project ID, file ID, and current SHA-1 are required")
	}

	file, err := client.GetFile(ctx, projectID, fileID)
	if err != nil {
		return VerifiedCurseForgeSource{}, err
	}
	if strconv.Itoa(file.ModID) != projectID {
		return VerifiedCurseForgeSource{}, fmt.Errorf("CurseForge file %d belongs to project %d, not %s", fileID, file.ModID, projectID)
	}
	if actual := curseForgeSHA1(file); !strings.EqualFold(actual, expectedSHA1) {
		return VerifiedCurseForgeSource{}, fmt.Errorf("CurseForge file %d SHA-1 does not match the installed JAR", fileID)
	}

	return VerifiedCurseForgeSource{
		ProjectID: projectID,
		FileID: fileID,
		DisplayName: file.DisplayName,
		Filename: file.FileName,
	}, nil
}

func (client *CurseForgeClient) ResolveInstalledFile(
	ctx context.Context,
	projectID string,
	minecraft string,
	loader string,
	expectedSHA1 string,
) (VerifiedCurseForgeSource, error) {
	expectedSHA1 = strings.ToLower(strings.TrimSpace(expectedSHA1))
	if strings.TrimSpace(projectID) == "" || expectedSHA1 == "" {
		return VerifiedCurseForgeSource{}, fmt.Errorf("project ID and current SHA-1 are required")
	}
	files, err := client.ListFiles(ctx, projectID, minecraft, loader)
	if err != nil {
		return VerifiedCurseForgeSource{}, err
	}
	var matched *curseForgeFile
	for index := range files {
		if !strings.EqualFold(curseForgeSHA1(files[index]), expectedSHA1) {
			continue
		}
		if matched != nil {
			return VerifiedCurseForgeSource{}, fmt.Errorf(
				"multiple CurseForge files for project %s match the installed JAR SHA-1",
				projectID,
			)
		}
		copy := files[index]
		matched = &copy
	}
	if matched == nil {
		return VerifiedCurseForgeSource{}, fmt.Errorf(
			"no compatible CurseForge file in project %s matches the installed JAR SHA-1",
			projectID,
		)
	}
	return VerifiedCurseForgeSource{
		ProjectID: projectID,
		FileID: uint32(matched.ID),
		DisplayName: matched.DisplayName,
		Filename: matched.FileName,
	}, nil
}

func (client *CurseForgeClient) MatchFingerprints(
	ctx context.Context,
	fingerprints []uint32,
) (map[uint32]inventory.CurseForgeMatch, error) {
	if strings.TrimSpace(client.APIKey) == "" {
		return nil, fmt.Errorf("CurseForge API key is not configured")
	}

	const batchSize = 100
	unique := make([]uint32, 0, len(fingerprints))
	seen := map[uint32]struct{}{}
	for _, fingerprint := range fingerprints {
		if fingerprint == 0 {
			continue
		}
		if _, exists := seen[fingerprint]; exists {
			continue
		}
		seen[fingerprint] = struct{}{}
		unique = append(unique, fingerprint)
	}

	result := make(map[uint32]inventory.CurseForgeMatch)
	for start := 0; start < len(unique); start += batchSize {
		end := start + batchSize
		if end > len(unique) {
			end = len(unique)
		}
		body, err := json.Marshal(map[string]any{
			"fingerprints": unique[start:end],
		})
		if err != nil {
			return nil, err
		}

		base := strings.TrimRight(client.BaseURL, "/")
		if base == "" {
			base = DefaultCurseForgeAPI
		}
		httpClient := client.HTTPClient
		if httpClient == nil {
			httpClient = &http.Client{Timeout: 30 * time.Second}
		}

		var response curseForgeFingerprintResponse
		if err := doJSONWithRetry(
			ctx,
			"curseforge",
			client.Mode,
			httpClient,
			func() (*http.Request, error) {
				request, err := http.NewRequestWithContext(
					ctx,
					http.MethodPost,
					base+"/fingerprints/432",
					bytes.NewReader(body),
				)
				if err != nil {
					return nil, err
				}
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Accept", "application/json")
				request.Header.Set("x-api-key", client.APIKey)
				request.Header.Set("User-Agent", "fpbcraft/fpbpack")
				return request, nil
			},
			&response,
		); err != nil {
			return nil, err
		}

		for _, exact := range response.Data.ExactMatches {
			file := exact.File
			fingerprint := file.FileFingerprint
			if fingerprint == 0 {
				fingerprint = exact.ID
			}
			if fingerprint == 0 || file.ID == 0 || file.ModID == 0 {
				continue
			}
			result[fingerprint] = inventory.CurseForgeMatch{
				ProjectID:    uint32(file.ModID),
				FileID:       uint32(file.ID),
				DisplayName:  file.DisplayName,
				Filename:     file.FileName,
				GameVersions: append([]string(nil), file.GameVersions...),
				ReleaseType:  file.ReleaseType,
			}
		}
	}
	return result, nil
}

func (client *CurseForgeClient) Validate(ctx context.Context) error {
	var response struct {
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	if err := client.getJSON(ctx, "/games/432", &response); err != nil {
		return err
	}
	if response.Data.ID != 432 {
		return fmt.Errorf("CurseForge credential validation returned an unexpected Minecraft game id")
	}
	return nil
}

func (client *CurseForgeClient) GetMod(ctx context.Context, projectID string) (curseForgeMod, error) {
	var response curseForgeModResponse
	if err := client.getJSON(ctx, "/mods/"+url.PathEscape(projectID), &response); err != nil {
		return curseForgeMod{}, err
	}
	return response.Data, nil
}

func (client *CurseForgeClient) GetFile(ctx context.Context, projectID string, fileID uint32) (curseForgeFile, error) {
	var response curseForgeFileResponse
	path := "/mods/" + url.PathEscape(projectID) + "/files/" + strconv.FormatUint(uint64(fileID), 10)
	if err := client.getJSON(ctx, path, &response); err != nil {
		return curseForgeFile{}, err
	}
	return response.Data, nil
}

func (client *CurseForgeClient) ListFiles(
	ctx context.Context,
	projectID string,
	minecraft string,
	loader string,
) ([]curseForgeFile, error) {
	const pageSize = 50
	result := make([]curseForgeFile, 0, pageSize)
	for index := 0; index < 10000; index += pageSize {
		values := url.Values{}
		values.Set("gameVersion", minecraft)
		if loaderType := curseForgeLoaderType(loader); loaderType != 0 {
			values.Set("modLoaderType", strconv.Itoa(loaderType))
		}
		values.Set("index", strconv.Itoa(index))
		values.Set("pageSize", strconv.Itoa(pageSize))

		var response curseForgeFilesResponse
		path := "/mods/" + url.PathEscape(projectID) + "/files?" + values.Encode()
		if err := client.getJSON(ctx, path, &response); err != nil {
			return nil, err
		}
		result = append(result, response.Data...)

		if response.Pagination.TotalCount > 0 {
			if index+len(response.Data) >= response.Pagination.TotalCount {
				break
			}
		} else if len(response.Data) < pageSize {
			break
		}
		if len(response.Data) == 0 {
			break
		}
	}
	return result, nil
}

func (client *CurseForgeClient) DownloadURL(ctx context.Context, projectID string, file curseForgeFile) (string, error) {
	if strings.TrimSpace(file.DownloadURL) != "" {
		return file.DownloadURL, nil
	}
	var response curseForgeStringResponse
	path := "/mods/" + url.PathEscape(projectID) + "/files/" + strconv.Itoa(file.ID) + "/download-url"
	if err := client.getJSON(ctx, path, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Data) == "" {
		return "", fmt.Errorf("CurseForge did not provide a download URL for file %d", file.ID)
	}
	return response.Data, nil
}

func (client *CurseForgeClient) Changelog(ctx context.Context, projectID string, fileID int) (string, error) {
	var response curseForgeStringResponse
	path := "/mods/" + url.PathEscape(projectID) + "/files/" + strconv.Itoa(fileID) + "/changelog"
	if err := client.getJSON(ctx, path, &response); err != nil {
		return "", err
	}
	return plainTextChangelog(response.Data), nil
}

func (client *CurseForgeClient) getJSON(ctx context.Context, path string, target any) error {
	if strings.TrimSpace(client.APIKey) == "" {
		return fmt.Errorf("CurseForge API key is not configured")
	}
	base := strings.TrimRight(client.BaseURL, "/")
	if base == "" {
		base = DefaultCurseForgeAPI
	}
	endpoint := base + path
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return doJSONWithRetry(
		ctx,
		"curseforge",
		client.Mode,
		httpClient,
		func() (*http.Request, error) {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				return nil, err
			}
			request.Header.Set("Accept", "application/json")
			request.Header.Set("x-api-key", client.APIKey)
			request.Header.Set("User-Agent", "fpbcraft/fpbpack")
			return request, nil
		},
		target,
	)
}

func curseForgeLoaderType(loader string) int {
	switch strings.ToLower(strings.TrimSpace(loader)) {
	case "forge":
		return 1
	case "fabric":
		return 4
	case "quilt":
		return 5
	case "neoforge":
		return 6
	default:
		return 0
	}
}

func curseForgeReleaseType(value int) string {
	switch value {
	case 1:
		return "release"
	case 2:
		return "beta"
	case 3:
		return "alpha"
	default:
		return "unknown"
	}
}

func curseForgeSHA1(file curseForgeFile) string {
	for _, hash := range file.Hashes {
		if hash.Algo == 1 {
			return strings.ToLower(strings.TrimSpace(hash.Value))
		}
	}
	return ""
}

var htmlTag = regexp.MustCompile(`<[^>]+>`)

func plainTextChangelog(value string) string {
	value = strings.ReplaceAll(value, "<br>", "\n")
	value = strings.ReplaceAll(value, "<br/>", "\n")
	value = strings.ReplaceAll(value, "<br />", "\n")
	value = strings.ReplaceAll(value, "</p>", "\n\n")
	value = strings.ReplaceAll(value, "</li>", "\n")
	value = htmlTag.ReplaceAllString(value, "")
	value = html.UnescapeString(value)
	lines := strings.Split(value, "\n")
	for index := range lines {
		lines[index] = strings.TrimSpace(lines[index])
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
