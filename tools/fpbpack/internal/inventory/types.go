package inventory

import "time"

const SchemaVersion = 1

type Location string

const (
	LocationServer Location = "server"
	LocationClient Location = "client"
)

type ModMetadata struct {
	Loader      string `json:"loader,omitempty"`
	ModID       string `json:"mod_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	SourceEntry string `json:"source_entry,omitempty"`
}

type ModrinthMatch struct {
	ProjectID     string   `json:"project_id"`
	VersionID     string   `json:"version_id"`
	VersionNumber string   `json:"version_number"`
	VersionName   string   `json:"version_name,omitempty"`
	Filename      string   `json:"filename,omitempty"`
	URL           string   `json:"url,omitempty"`
	Loaders       []string `json:"loaders,omitempty"`
	GameVersions  []string `json:"game_versions,omitempty"`
	Environment   string   `json:"environment,omitempty"`
}

type ModFile struct {
	Location Location       `json:"location"`
	Path     string         `json:"path"`
	Filename string         `json:"filename"`
	Size     int64          `json:"size"`
	SHA1     string         `json:"sha1"`
	SHA512   string         `json:"sha512"`
	Metadata []ModMetadata  `json:"metadata,omitempty"`
	Modrinth *ModrinthMatch `json:"modrinth,omitempty"`
	Error    string         `json:"error,omitempty"`
}

type Summary struct {
	Total              int `json:"total"`
	Server             int `json:"server"`
	Client             int `json:"client"`
	ModrinthExact      int `json:"modrinth_exact"`
	Unmatched          int `json:"unmatched"`
	MetadataUnreadable int `json:"metadata_unreadable"`
}

type Inventory struct {
	SchemaVersion   int       `json:"schema_version"`
	GeneratedAt     time.Time `json:"generated_at"`
	ServerRoot      string    `json:"server_root"`
	ServerModsPath  string    `json:"server_mods_path"`
	ClientModsPath  string    `json:"client_mods_path"`
	ModrinthChecked bool      `json:"modrinth_checked"`
	ModrinthError   string    `json:"modrinth_error,omitempty"`
	Warnings        []string  `json:"warnings,omitempty"`
	Summary         Summary   `json:"summary"`
	Mods            []ModFile `json:"mods"`
}

func (i *Inventory) RecalculateSummary() {
	var summary Summary
	for _, mod := range i.Mods {
		summary.Total++
		switch mod.Location {
		case LocationServer:
			summary.Server++
		case LocationClient:
			summary.Client++
		}
		if mod.Modrinth != nil {
			summary.ModrinthExact++
		}
		if i.ModrinthChecked && mod.Modrinth == nil {
			summary.Unmatched++
		}
		if mod.Error != "" {
			summary.MetadataUnreadable++
		}
	}
	i.Summary = summary
}
