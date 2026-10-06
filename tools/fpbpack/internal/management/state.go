package management

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/doctor"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

type Source struct {
	InventoryPath string
	ReportPath    string
}

type Snapshot struct {
	Inventory   inventory.Inventory `json:"inventory"`
	Diagnostics doctor.Report       `json:"diagnostics"`
	Mods        []Mod               `json:"mods"`
	Status      Status              `json:"status"`
}

type Status struct {
	Mode                 string         `json:"mode"`
	ReadOnly             bool           `json:"read_only"`
	ServerState          string         `json:"server_state"`
	InventoryGeneratedAt time.Time      `json:"inventory_generated_at"`
	Mods                 int            `json:"mods"`
	Managed              int            `json:"managed"`
	Unmanaged            int            `json:"unmanaged"`
	Diagnostics          doctor.Summary `json:"diagnostics"`
}

type Mod struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Filename         string             `json:"filename"`
	InstalledVersion string             `json:"installed_version,omitempty"`
	Provider         string             `json:"provider,omitempty"`
	ProjectID        string             `json:"project_id,omitempty"`
	ProjectURL       string             `json:"project_url,omitempty"`
	Side             string             `json:"side"`
	Deployment          inventory.Location `json:"deployment"`
	AutoModpackGroup    string             `json:"automodpack_group,omitempty"`
	PreferredDeployment inventory.Location `json:"preferred_deployment"`
	PreferredAutoModpackGroup string       `json:"preferred_automodpack_group,omitempty"`
	Management       string             `json:"management"`
	Path             string             `json:"path"`
	SHA512           string             `json:"sha512,omitempty"`
}

func (s Source) Load() (Snapshot, error) {
	if strings.TrimSpace(s.InventoryPath) == "" {
		return Snapshot{}, fmt.Errorf("inventory path is required")
	}
	if strings.TrimSpace(s.ReportPath) == "" {
		return Snapshot{}, fmt.Errorf("migration report path is required")
	}

	var inv inventory.Inventory
	if err := readJSON(s.InventoryPath, &inv); err != nil {
		return Snapshot{}, fmt.Errorf("load inventory: %w", err)
	}
	var cat catalog.Report
	if err := readJSON(s.ReportPath, &cat); err != nil {
		return Snapshot{}, fmt.Errorf("load migration report: %w", err)
	}

	return BuildSnapshot(inv, cat), nil
}

func BuildSnapshot(inv inventory.Inventory, cat catalog.Report) Snapshot {
	diagnostics := doctor.Analyze(inv, cat)
	mods := BuildMods(inv, cat)
	status := Status{
		Mode:                 "read-only",
		ReadOnly:             true,
		ServerState:          "unknown",
		InventoryGeneratedAt: inv.GeneratedAt,
		Mods:                 len(mods),
		Managed:              len(cat.Managed),
		Unmanaged:            len(cat.Pinned),
		Diagnostics:          diagnostics.Summary,
	}
	return Snapshot{
		Inventory:   inv,
		Diagnostics: diagnostics,
		Mods:        mods,
		Status:      status,
	}
}

func BuildMods(inv inventory.Inventory, cat catalog.Report) []Mod {
	managedByHash := make(map[string]catalog.Entry, len(cat.Managed))
	managedByPath := map[string]catalog.Entry{}
	pinnedByHash := make(map[string]catalog.PinnedArtifact, len(cat.Pinned))
	pinnedByPath := map[string]catalog.PinnedArtifact{}
	unresolvedByHash := make(map[string]catalog.Unresolved, len(cat.Unresolved))
	unresolvedByPath := map[string]catalog.Unresolved{}
	for _, entry := range cat.Managed {
		managedByHash[strings.ToLower(entry.SHA512)] = entry
		for _, source := range entry.SourcePaths {
			managedByPath[statePathKey(source.Location, source.Path)] = entry
		}
	}
	for _, entry := range cat.Pinned {
		pinnedByHash[strings.ToLower(entry.SHA512)] = entry
		for _, source := range entry.Sources {
			pinnedByPath[statePathKey(source.Location, source.Path)] = entry
		}
	}
	for _, entry := range cat.Unresolved {
		unresolvedByHash[strings.ToLower(entry.SHA512)] = entry
		for _, source := range entry.Sources {
			unresolvedByPath[statePathKey(source.Location, source.Path)] = entry
		}
	}

	mods := make([]Mod, 0, len(inv.Mods))
	for _, file := range inv.Mods {
		hashKey := strings.ToLower(file.SHA512)
		pathKey := statePathKey(file.Location, file.Path)
		managed, isManaged := managedByHash[hashKey]
		if !isManaged {
			managed, isManaged = managedByPath[pathKey]
		}
		_, isPinned := pinnedByHash[hashKey]
		if !isPinned {
			_, isPinned = pinnedByPath[pathKey]
		}
		_, isUnresolved := unresolvedByHash[hashKey]
		if !isUnresolved {
			_, isUnresolved = unresolvedByPath[pathKey]
		}

		currentGroup := file.Group
		if file.Location == inventory.LocationClient && strings.TrimSpace(currentGroup) == "" {
			currentGroup = "main"
		}
		mod := Mod{
			ID:               modID(file, managed, isManaged),
			Name:             displayName(file, managed, isManaged),
			Filename:         file.Filename,
			InstalledVersion: displayVersion(file),
			Side:             string(file.Location),
			Deployment:       file.Location,
			AutoModpackGroup: currentGroup,
			PreferredDeployment: file.Location,
			PreferredAutoModpackGroup: currentGroup,
			Management:       "unresolved",
			Path:             file.Path,
			SHA512:           file.SHA512,
		}

		switch {
		case isManaged:
			mod.Management = "managed"
			if managed.Deployment != "" {
				mod.PreferredDeployment = managed.Deployment
			}
			if managed.Deployment == inventory.LocationClient {
				mod.PreferredAutoModpackGroup = strings.TrimSpace(managed.AutoModpackGroup)
				if mod.PreferredAutoModpackGroup == "" {
					mod.PreferredAutoModpackGroup = "main"
				}
			} else {
				mod.PreferredAutoModpackGroup = ""
			}
			mod.Provider = managed.Provider
			mod.ProjectID = managed.ProjectID
			if managed.Side != "" {
				mod.Side = managed.Side
			}
			mod.ProjectURL = projectURL(managed)
		case isPinned:
			mod.Management = "unmanaged"
			mod.Provider = "local"
		case isUnresolved:
			mod.Management = "unresolved"
		default:
			mod.Management = "external"
		}
		mods = append(mods, mod)
	}
	return mods
}

func readJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(target); err != nil {
		return err
	}
	return nil
}

func displayName(file inventory.ModFile, managed catalog.Entry, isManaged bool) string {
	if isManaged && strings.TrimSpace(managed.Name) != "" {
		return managed.Name
	}
	for _, meta := range file.Metadata {
		if strings.TrimSpace(meta.Name) != "" {
			return meta.Name
		}
	}
	return file.Filename
}

func displayVersion(file inventory.ModFile) string {
	for _, meta := range file.Metadata {
		if strings.TrimSpace(meta.Version) != "" {
			return meta.Version
		}
	}
	if file.Modrinth != nil && strings.TrimSpace(file.Modrinth.VersionNumber) != "" {
		return file.Modrinth.VersionNumber
	}
	if file.CurseForge != nil && strings.TrimSpace(file.CurseForge.DisplayName) != "" {
		return file.CurseForge.DisplayName
	}
	return ""
}

func modID(file inventory.ModFile, managed catalog.Entry, isManaged bool) string {
	if isManaged && managed.Provider != "" && managed.ProjectID != "" {
		return catalog.EntryKey(managed)
	}
	if file.SHA512 != "" {
		hash := strings.ToLower(file.SHA512)
		if len(hash) > 16 {
			hash = hash[:16]
		}
		return "sha512:" + hash
	}
	return string(file.Location) + ":" + file.Path
}

func projectURL(entry catalog.Entry) string {
	switch entry.Provider {
	case "modrinth":
		if entry.ProjectID != "" {
			return "https://modrinth.com/project/" + entry.ProjectID
		}
	case "github":
		if entry.ProjectID != "" {
			return "https://github.com/" + entry.ProjectID
		}
	}
	return ""
}

func statePathKey(location inventory.Location, path string) string {
	return string(location) + ":" + strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "./")
}
