package inventory

import "time"

const SchemaVersion = 2

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

type CurseForgeMatch struct {
	ProjectID    uint32   `json:"project_id"`
	FileID       uint32   `json:"file_id"`
	DisplayName  string   `json:"display_name,omitempty"`
	Filename     string   `json:"filename,omitempty"`
	GameVersions []string `json:"game_versions,omitempty"`
	ReleaseType  int      `json:"release_type,omitempty"`
}

type ModFile struct {
	Location              Location         `json:"location"`
	Group                 string           `json:"group,omitempty"`
	Path                  string           `json:"path"`
	Filename              string           `json:"filename"`
	Size                  int64            `json:"size"`
	ModifiedUnixNano      int64            `json:"modified_unix_nano,omitempty"`
	Cached                bool             `json:"-"`
	SHA1                  string           `json:"sha1"`
	SHA512                string           `json:"sha512"`
	CurseForgeFingerprint uint32           `json:"curseforge_fingerprint,omitempty"`
	Metadata              []ModMetadata    `json:"metadata,omitempty"`
	Modrinth              *ModrinthMatch   `json:"modrinth,omitempty"`
	CurseForge            *CurseForgeMatch `json:"curseforge,omitempty"`
	Error                 string           `json:"error,omitempty"`
}

type Summary struct {
	Total              int `json:"total"`
	Server             int `json:"server"`
	Client             int `json:"client"`
	ClientGroups       int `json:"client_groups,omitempty"`
	ModrinthExact      int `json:"modrinth_exact"`
	CurseForgeExact    int `json:"curseforge_exact"`
	Unmatched          int `json:"unmatched"`
	MetadataUnreadable int `json:"metadata_unreadable"`
}

type Inventory struct {
	SchemaVersion      int       `json:"schema_version"`
	GeneratedAt        time.Time `json:"generated_at"`
	ServerRoot         string    `json:"server_root"`
	ServerModsPath     string    `json:"server_mods_path"`
	ClientModsPath     string    `json:"client_mods_path"`
	ClientGroupModsPaths map[string]string `json:"client_group_mods_paths,omitempty"`
	ModrinthChecked    bool      `json:"modrinth_checked"`
	ModrinthError      string    `json:"modrinth_error,omitempty"`
	CurseForgeChecked  bool      `json:"curseforge_checked"`
	CurseForgeError    string    `json:"curseforge_error,omitempty"`
	Warnings           []string  `json:"warnings,omitempty"`
	Summary            Summary   `json:"summary"`
	Mods               []ModFile `json:"mods"`
}

func (i *Inventory) RecalculateSummary() {
	var summary Summary
	clientGroups := map[string]struct{}{}
	for _, mod := range i.Mods {
		summary.Total++
		switch mod.Location {
		case LocationServer:
			summary.Server++
		case LocationClient:
			summary.Client++
			group := mod.Group
			if group == "" {
				group = "main"
			}
			clientGroups[group] = struct{}{}
		}
		if mod.Modrinth != nil {
			summary.ModrinthExact++
		}
		if mod.CurseForge != nil {
			summary.CurseForgeExact++
		}
		if i.CurseForgeChecked {
			if mod.Modrinth == nil && mod.CurseForge == nil {
				summary.Unmatched++
			}
		} else if i.ModrinthChecked && mod.Modrinth == nil {
			summary.Unmatched++
		}
		if mod.Error != "" {
			summary.MetadataUnreadable++
		}
	}
	summary.ClientGroups = len(clientGroups)
	i.Summary = summary
}
