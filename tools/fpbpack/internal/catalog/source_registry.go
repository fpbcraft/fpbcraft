package catalog

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

const SourceRegistrySchemaVersion = 1

type SourceRegistry struct {
	SchemaVersion int          `json:"schema_version"`
	Sources       []SourceRule `json:"sources"`
}

type SourceRule struct {
	SHA512     string `json:"sha512"`
	Type       string `json:"type"`
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Asset      string `json:"asset,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type SourceResolutionSummary struct {
	GitHubVerified int
	Pinned         int
	Remaining      int
}

type SourceResolveOptions struct {
	RegistryPath  string
	OutputPath    string
	PackwizPath   string
	GitHubBaseURL string
	HTTPClient    *http.Client
}

func ResolveSourceRegistry(ctx context.Context, inv inventory.Inventory, result *Result, opts SourceResolveOptions) (SourceResolutionSummary, error) {
	var summary SourceResolutionSummary
	registry, err := loadSourceRegistry(opts.RegistryPath)
	if err != nil {
		return summary, err
	}
	rules := make(map[string]SourceRule, len(registry.Sources))
	for _, rule := range registry.Sources {
		key := strings.ToLower(strings.TrimSpace(rule.SHA512))
		if len(key) != 128 {
			return summary, fmt.Errorf("source registry has invalid SHA-512 %q", rule.SHA512)
		}
		if _, exists := rules[key]; exists {
			return summary, fmt.Errorf("source registry has duplicate SHA-512 %s", key)
		}
		rule.SHA512 = key
		rules[key] = rule
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	baseURL := strings.TrimRight(opts.GitHubBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://github.com"
	}

	remaining := make([]Unresolved, 0, len(result.Report.Unresolved))
	addedGitHub := false
	for _, unresolved := range result.Report.Unresolved {
		rule, ok := rules[strings.ToLower(unresolved.SHA512)]
		if !ok {
			remaining = append(remaining, unresolved)
			continue
		}

		switch rule.Type {
		case "pinned_local":
			if strings.TrimSpace(rule.Reason) == "" {
				return summary, fmt.Errorf("pinned_local source for %s requires a reason", unresolved.Filename)
			}
			result.Report.Pinned = append(result.Report.Pinned, PinnedArtifact{
				SHA512: unresolved.SHA512, Filename: unresolved.Filename,
				Reason: rule.Reason, Sources: unresolved.Sources,
			})
			summary.Pinned++

		case "github_release":
			if err := validateGitHubRule(rule); err != nil {
				return summary, fmt.Errorf("%s: %w", unresolved.Filename, err)
			}
			downloadURL := githubReleaseURL(baseURL, rule)
			remoteHash, err := sha512URL(ctx, client, downloadURL)
			if err != nil {
				return summary, fmt.Errorf("verify GitHub source for %s: %w", unresolved.Filename, err)
			}
			if !strings.EqualFold(remoteHash, unresolved.SHA512) {
				return summary, fmt.Errorf("GitHub source hash mismatch for %s: installed=%s remote=%s (%s)", unresolved.Filename, unresolved.SHA512, remoteHash, downloadURL)
			}
			mod := findInventoryArtifact(inv, unresolved.SHA512, unresolved.Filename)
			deployment := inventory.LocationServer
			if len(unresolved.Sources) > 0 {
				deployment = unresolved.Sources[0].Location
			}
			entry := Entry{
				Provider: "github",
				ProjectID: rule.Repository,
				Name: displayName(mod),
				Filename: unresolved.Filename,
				SHA1: mod.SHA1,
				SHA512: unresolved.SHA512,
				URL: downloadURL,
				Side: packwizSide("", deployment),
				Deployment: deployment,
				Repository: rule.Repository,
				Tag: rule.Tag,
				Asset: rule.Asset,
				SourcePaths: unresolved.Sources,
			}
			result.Entries = append(result.Entries, entry)
			result.Report.Managed = append(result.Report.Managed, entry)
			metaPath := filepath.Join(opts.OutputPath, "mods", metafileName(entry))
			if err := os.WriteFile(metaPath, []byte(renderMetafile(entry)), 0o644); err != nil {
				return summary, fmt.Errorf("write GitHub metafile for %s: %w", unresolved.Filename, err)
			}
			summary.GitHubVerified++
			addedGitHub = true

		default:
			return summary, fmt.Errorf("unsupported source registry type %q for %s", rule.Type, unresolved.Filename)
		}
	}

	result.Report.Unresolved = remaining
	sort.Slice(result.Report.Pinned, func(i, j int) bool {
		return result.Report.Pinned[i].Filename < result.Report.Pinned[j].Filename
	})
	sort.Slice(result.Entries, func(i, j int) bool {
		if result.Entries[i].Provider != result.Entries[j].Provider {
			return result.Entries[i].Provider < result.Entries[j].Provider
		}
		if result.Entries[i].ProjectID != result.Entries[j].ProjectID {
			return result.Entries[i].ProjectID < result.Entries[j].ProjectID
		}
		return result.Entries[i].Filename < result.Entries[j].Filename
	})
	sort.Slice(result.Report.Managed, func(i, j int) bool {
		if result.Report.Managed[i].Provider != result.Report.Managed[j].Provider {
			return result.Report.Managed[i].Provider < result.Report.Managed[j].Provider
		}
		if result.Report.Managed[i].ProjectID != result.Report.Managed[j].ProjectID {
			return result.Report.Managed[i].ProjectID < result.Report.Managed[j].ProjectID
		}
		return result.Report.Managed[i].Filename < result.Report.Managed[j].Filename
	})
	result.Report.Summary.GeneratedProjects = len(result.Report.Managed)
	result.Report.Summary.Unresolved = len(result.Report.Unresolved)
	result.Report.Summary.PinnedArtifacts = len(result.Report.Pinned)
	summary.Remaining = len(result.Report.Unresolved)

	if addedGitHub {
		executable, err := resolveExecutable(opts.PackwizPath)
		if err != nil {
			return summary, err
		}
		refreshOutput, err := runPackwiz(executable, opts.OutputPath, "refresh")
		if err != nil {
			return summary, fmt.Errorf("packwiz refresh after GitHub source resolution failed: %w\n%s", err, strings.TrimSpace(refreshOutput))
		}
	}
	if err := rewriteMigrationReport(opts.OutputPath, result.Report); err != nil {
		return summary, err
	}
	return summary, nil
}

func loadSourceRegistry(path string) (SourceRegistry, error) {
	if strings.TrimSpace(path) == "" {
		return SourceRegistry{}, fmt.Errorf("source registry path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return SourceRegistry{}, fmt.Errorf("open source registry: %w", err)
	}
	defer file.Close()
	var registry SourceRegistry
	if err := json.NewDecoder(file).Decode(&registry); err != nil {
		return SourceRegistry{}, fmt.Errorf("decode source registry: %w", err)
	}
	if registry.SchemaVersion != SourceRegistrySchemaVersion {
		return SourceRegistry{}, fmt.Errorf("unsupported source registry schema %d (expected %d)", registry.SchemaVersion, SourceRegistrySchemaVersion)
	}
	return registry, nil
}

func validateGitHubRule(rule SourceRule) error {
	if strings.Count(rule.Repository, "/") != 1 || strings.TrimSpace(rule.Repository) == "" {
		return fmt.Errorf("github_release requires repository in owner/name form")
	}
	if strings.TrimSpace(rule.Tag) == "" {
		return fmt.Errorf("github_release requires tag")
	}
	if strings.TrimSpace(rule.Asset) == "" {
		return fmt.Errorf("github_release requires asset")
	}
	return nil
}

func githubReleaseURL(baseURL string, rule SourceRule) string {
	parts := strings.SplitN(rule.Repository, "/", 2)
	return strings.TrimRight(baseURL, "/") + "/" +
		url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) +
		"/releases/download/" + url.PathEscape(rule.Tag) + "/" + url.PathEscape(rule.Asset)
}

func sha512URL(ctx context.Context, client *http.Client, downloadURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "fpbpack/source-resolver")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("GET %s returned HTTP %d", downloadURL, resp.StatusCode)
	}
	hash := sha512.New()
	if _, err := io.Copy(hash, resp.Body); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
