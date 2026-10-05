package updates

import (
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

type CatalogProject struct {
	Provider    string   `json:"provider"`
	ProjectID   string   `json:"project_id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	IconURL     string   `json:"icon_url,omitempty"`
	ProjectURL  string   `json:"project_url,omitempty"`
	Downloads   int64    `json:"downloads,omitempty"`
	Environment []string `json:"environment,omitempty"`
	Installed   bool     `json:"installed"`
	InstalledKey string  `json:"installed_key,omitempty"`
}

type CatalogVersion struct {
	ID           string    `json:"id"`
	Number       string    `json:"number"`
	Name         string    `json:"name,omitempty"`
	PublishedAt  time.Time `json:"published_at,omitempty"`
	Channel      string    `json:"channel,omitempty"`
	Filename     string    `json:"filename,omitempty"`
	Environment  string    `json:"environment,omitempty"`
	Changelog    string    `json:"changelog,omitempty"`
	SHA1         string    `json:"sha1,omitempty"`
	SHA512       string    `json:"sha512,omitempty"`
	ManualDownload bool   `json:"manual_download,omitempty"`
	ManualURL    string    `json:"manual_url,omitempty"`
}

type Classification string

const (
	ClassificationSafe     Classification = "safe"
	ClassificationReview   Classification = "review"
	ClassificationBlocked  Classification = "blocked"
	ClassificationIgnored  Classification = "ignored"
	ClassificationUpToDate Classification = "up_to_date"
)

type Reason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Release struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Name        string    `json:"name,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	Channel     string    `json:"channel,omitempty"`
	Filename    string    `json:"filename,omitempty"`
	URL         string    `json:"url,omitempty"`
	SHA1        string    `json:"sha1,omitempty"`
	SHA256      string    `json:"sha256,omitempty"`
	SHA512      string    `json:"sha512,omitempty"`
	ManualDownload bool   `json:"manual_download,omitempty"`
	ManualURL     string   `json:"manual_url,omitempty"`
}

type Dependency struct {
	Provider         string             `json:"provider"`
	ProjectID        string             `json:"project_id,omitempty"`
	VersionID        string             `json:"version_id,omitempty"`
	Name             string             `json:"name,omitempty"`
	Type             string             `json:"type"`
	Action           string             `json:"action"`
	InstalledVersion string             `json:"installed_version,omitempty"`
	TargetVersion    string             `json:"target_version,omitempty"`
	Deployment       inventory.Location `json:"deployment,omitempty"`
	Target           *Release           `json:"target,omitempty"`
	Dependencies     []Dependency       `json:"dependencies,omitempty"`
}

type ChangelogEntry struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Name        string    `json:"name,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	Channel     string    `json:"channel,omitempty"`
	Body        string    `json:"body,omitempty"`
}

type RejectedVersion struct {
	ID      string   `json:"id"`
	Number  string   `json:"number"`
	Reasons []Reason `json:"reasons"`
}

type Candidate struct {
	Key            string             `json:"key"`
	Provider       string             `json:"provider"`
	ProjectID      string             `json:"project_id"`
	Name           string             `json:"name"`
	ProjectURL     string             `json:"project_url,omitempty"`
	IconURL        string             `json:"icon_url,omitempty"`
	Side           string             `json:"side"`
	Deployment     inventory.Location `json:"deployment"`
	Installed      Release            `json:"installed"`
	Target         *Release           `json:"target,omitempty"`
	Classification     Classification     `json:"classification"`
	BaseClassification Classification     `json:"base_classification,omitempty"`
	Reasons        []Reason           `json:"reasons,omitempty"`
	Dependencies   []Dependency       `json:"dependencies,omitempty"`
	Changelogs     []ChangelogEntry   `json:"changelogs,omitempty"`
	RequiredBy     []string           `json:"required_by,omitempty"`
	MetadataStale  bool               `json:"metadata_stale,omitempty"`
	RefreshError   string             `json:"refresh_error,omitempty"`
	Rejected       []RejectedVersion  `json:"rejected,omitempty"`
	Intent         string             `json:"intent,omitempty"`
}

type Summary struct {
	Safe     int `json:"safe"`
	Review   int `json:"review"`
	Blocked  int `json:"blocked"`
	Ignored  int `json:"ignored"`
	UpToDate int `json:"up_to_date"`
}

type Report struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Minecraft   string      `json:"minecraft"`
	Loader      string      `json:"loader"`
	Summary     Summary     `json:"summary"`
	Candidates  []Candidate `json:"candidates"`
}

func (r *Report) RecalculateSummary() {
	if r.Candidates == nil {
		r.Candidates = []Candidate{}
	}
	r.Summary = Summary{}
	for _, candidate := range r.Candidates {
		switch candidate.Classification {
		case ClassificationSafe:
			r.Summary.Safe++
		case ClassificationReview:
			r.Summary.Review++
		case ClassificationBlocked:
			r.Summary.Blocked++
		case ClassificationIgnored:
			r.Summary.Ignored++
		case ClassificationUpToDate:
			r.Summary.UpToDate++
		}
	}
}
