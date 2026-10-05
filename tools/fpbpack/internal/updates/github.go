package updates

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
)

const DefaultGitHubAPI = "https://api.github.com"

type GitHubClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	Mode       RefreshMode
}

type githubRelease struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		ID                 int64  `json:"id"`
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Digest             string `json:"digest"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

func (client *GitHubClient) Releases(ctx context.Context, repository string) ([]githubRelease, error) {
	if strings.Count(repository, "/") != 1 {
		return nil, fmt.Errorf("GitHub repository must be owner/name")
	}
	base := strings.TrimRight(client.BaseURL, "/")
	if base == "" {
		base = DefaultGitHubAPI
	}
	parts := strings.SplitN(repository, "/", 2)
	endpoint := base + "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/releases?per_page=50"
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	var releases []githubRelease
	err := doJSONWithRetry(
		ctx,
		"github",
		client.Mode,
		httpClient,
		func() (*http.Request, error) {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				return nil, err
			}
			request.Header.Set("Accept", "application/vnd.github+json")
			request.Header.Set("User-Agent", "fpbcraft/fpbpack")
			request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
			if strings.TrimSpace(client.Token) != "" {
				request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(client.Token))
			}
			return request, nil
		},
		&releases,
	)
	if err != nil {
		return nil, fmt.Errorf("GitHub releases: %w", err)
	}
	return releases, nil
}

type VerifiedGitHubSource struct {
	Repository  string
	Tag         string
	Asset       string
	DownloadURL string
	Name        string
}

func (client *GitHubClient) ResolveInstalledAsset(
	ctx context.Context,
	repository string,
	expectedSHA256 string,
) (VerifiedGitHubSource, error) {
	repository = strings.TrimSpace(repository)
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if repository == "" || expectedSHA256 == "" {
		return VerifiedGitHubSource{}, fmt.Errorf("repository and current SHA-256 are required")
	}
	releases, err := client.Releases(ctx, repository)
	if err != nil {
		return VerifiedGitHubSource{}, err
	}
	var matched *VerifiedGitHubSource
	for _, release := range releases {
		if release.Draft {
			continue
		}
		for _, asset := range release.Assets {
			if !strings.HasSuffix(strings.ToLower(asset.Name), ".jar") {
				continue
			}
			digest := strings.ToLower(githubSHA256Digest(asset.Digest))
			if digest == "" || digest != expectedSHA256 {
				continue
			}
			if matched != nil {
				return VerifiedGitHubSource{}, fmt.Errorf(
					"multiple GitHub release assets in %s match the installed JAR SHA-256",
					repository,
				)
			}
			name := release.Name
			if name == "" {
				name = repository
			}
			value := VerifiedGitHubSource{
				Repository: repository,
				Tag: release.TagName,
				Asset: asset.Name,
				DownloadURL: asset.BrowserDownloadURL,
				Name: name,
			}
			matched = &value
		}
	}
	if matched == nil {
		return VerifiedGitHubSource{}, fmt.Errorf(
			"no GitHub release asset in %s matches the installed JAR SHA-256",
			repository,
		)
	}
	return *matched, nil
}

func (client *GitHubClient) VerifyInstalledAsset(
	ctx context.Context,
	repository string,
	tag string,
	assetName string,
	expectedSHA256 string,
) (VerifiedGitHubSource, error) {
	repository = strings.TrimSpace(repository)
	tag = strings.TrimSpace(tag)
	assetName = strings.TrimSpace(assetName)
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if repository == "" || tag == "" || expectedSHA256 == "" {
		return VerifiedGitHubSource{}, fmt.Errorf("repository, installed tag, and current SHA-256 are required")
	}

	releases, err := client.Releases(ctx, repository)
	if err != nil {
		return VerifiedGitHubSource{}, err
	}
	for _, release := range releases {
		if release.TagName != tag {
			continue
		}
		asset, ok := selectGitHubAsset(release, assetName)
		if !ok {
			return VerifiedGitHubSource{}, fmt.Errorf("GitHub release %s does not contain an unambiguous matching JAR asset", tag)
		}
		digest := strings.ToLower(githubSHA256Digest(asset.Digest))
		if digest == "" {
			return VerifiedGitHubSource{}, fmt.Errorf("GitHub asset %s does not expose a SHA-256 digest", asset.Name)
		}
		if digest != expectedSHA256 {
			return VerifiedGitHubSource{}, fmt.Errorf("GitHub asset digest does not match the installed JAR")
		}
		name := release.Name
		if name == "" {
			name = repository
		}
		return VerifiedGitHubSource{
			Repository: repository,
			Tag: tag,
			Asset: asset.Name,
			DownloadURL: asset.BrowserDownloadURL,
			Name: name,
		}, nil
	}
	return VerifiedGitHubSource{}, fmt.Errorf("GitHub release tag %s was not found in %s", tag, repository)
}

func discoverGitHubCandidate(
	ctx context.Context,
	client *GitHubClient,
	entry catalog.Entry,
) Candidate {
	repository := strings.TrimSpace(entry.Repository)
	if repository == "" {
		repository = strings.TrimSpace(entry.ProjectID)
	}
	candidate := Candidate{
		Key: catalog.EntryKey(entry),
		Provider: "github",
		ProjectID: repository,
		Name: entry.Name,
		ProjectURL: "https://github.com/" + repository,
		Side: entry.Side,
		Deployment: entry.Deployment,
		Installed: installedRelease(entry),
	}
	candidate.Installed.ID = entry.Tag
	candidate.Installed.Number = entry.Tag

	if repository == "" || entry.Tag == "" {
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{
			Code: "github_source_incomplete",
			Message: "The verified GitHub source is missing repository or installed tag metadata.",
		}}
		return candidate
	}

	releases, err := client.Releases(ctx, repository)
	if err != nil {
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{Code: "provider_lookup_failed", Message: err.Error()}}
		return candidate
	}
	var current *githubRelease
	for index := range releases {
		if releases[index].TagName == entry.Tag {
			copy := releases[index]
			current = &copy
			break
		}
	}
	if current == nil {
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{
			Code: "github_installed_tag_not_found",
			Message: "The installed verified GitHub tag is not present in the repository release history.",
		}}
		return candidate
	}
	candidate.Installed.PublishedAt = current.PublishedAt
	if current.Name != "" {
		candidate.Installed.Name = current.Name
	}

	newer := make([]githubRelease, 0)
	for _, release := range releases {
		if release.Draft || !release.PublishedAt.After(current.PublishedAt) {
			continue
		}
		newer = append(newer, release)
	}
	if len(newer) == 0 {
		candidate.Classification = ClassificationUpToDate
		return candidate
	}
	sort.Slice(newer, func(i, j int) bool {
		return newer[i].PublishedAt.After(newer[j].PublishedAt)
	})
	targetRelease := newer[0]
	asset, ok := selectGitHubAsset(targetRelease, entry.Asset)
	if !ok {
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{
			Code: "github_target_asset_ambiguous",
			Message: "The newest GitHub release does not have an unambiguous JAR asset matching the verified source.",
		}}
		return candidate
	}
	sha256 := githubSHA256Digest(asset.Digest)
	if sha256 == "" {
		candidate.Classification = ClassificationBlocked
		candidate.Reasons = []Reason{{
			Code: "github_target_digest_missing",
			Message: "The GitHub release asset does not expose a SHA-256 digest, so FPBPack will not plan it.",
		}}
		return candidate
	}

	target := Release{
		ID: targetRelease.TagName,
		Number: targetRelease.TagName,
		Name: targetRelease.Name,
		PublishedAt: targetRelease.PublishedAt,
		Channel: "release",
		Filename: asset.Name,
		URL: asset.BrowserDownloadURL,
		SHA256: sha256,
	}
	if targetRelease.Prerelease {
		target.Channel = "prerelease"
	}
	candidate.Target = &target
	candidate.Classification = ClassificationReview
	candidate.Reasons = append(candidate.Reasons, Reason{
		Code: "github_release_requires_review",
		Message: "GitHub releases do not provide Minecraft/loader compatibility metadata; review is required.",
	})

	ascending := append([]githubRelease(nil), newer...)
	sort.Slice(ascending, func(i, j int) bool {
		return ascending[i].PublishedAt.Before(ascending[j].PublishedAt)
	})
	for _, release := range ascending {
		channel := "release"
		if release.Prerelease {
			channel = "prerelease"
		}
		candidate.Changelogs = append(candidate.Changelogs, ChangelogEntry{
			ID: release.TagName,
			Number: release.TagName,
			Name: release.Name,
			PublishedAt: release.PublishedAt,
			Channel: channel,
			Body: strings.TrimSpace(release.Body),
		})
	}
	return candidate
}

type githubAsset struct {
	Name               string
	BrowserDownloadURL string
	Digest             string
}

func selectGitHubAsset(release githubRelease, currentAsset string) (githubAsset, bool) {
	for _, asset := range release.Assets {
		if currentAsset != "" && asset.Name == currentAsset {
			return githubAsset{Name: asset.Name, BrowserDownloadURL: asset.BrowserDownloadURL, Digest: asset.Digest}, true
		}
	}
	jars := make([]githubAsset, 0)
	for _, asset := range release.Assets {
		if strings.HasSuffix(strings.ToLower(asset.Name), ".jar") {
			jars = append(jars, githubAsset{Name: asset.Name, BrowserDownloadURL: asset.BrowserDownloadURL, Digest: asset.Digest})
		}
	}
	if len(jars) == 1 {
		return jars[0], true
	}
	return githubAsset{}, false
}

func githubSHA256Digest(digest string) string {
	const prefix = "sha256:"
	digest = strings.TrimSpace(digest)
	if !strings.HasPrefix(strings.ToLower(digest), prefix) {
		return ""
	}
	return digest[len(prefix):]
}
