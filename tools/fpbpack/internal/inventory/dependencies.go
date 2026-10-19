package inventory

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func parseModsTOML(content, loader, sourceEntry, manifestVersion string) ([]ModMetadata, error) {
	var raw struct {
		Mods []struct {
			ModID    string   `toml:"modId"`
			Name     string   `toml:"displayName"`
			Version  string   `toml:"version"`
			Provides []string `toml:"provides"`
		} `toml:"mods"`
		Dependencies map[string][]struct {
			ModID        string `toml:"modId"`
			Type         string `toml:"type"`
			Mandatory    *bool  `toml:"mandatory"`
			VersionRange string `toml:"versionRange"`
			Side         string `toml:"side"`
		} `toml:"dependencies"`
	}
	if err := toml.Unmarshal([]byte(content), &raw); err != nil {
		return nil, err
	}
	var mods []ModMetadata
	for _, m := range raw.Mods {
		if m.ModID == "" {
			continue
		}
		if m.Version == "${file.jarVersion}" && manifestVersion != "" {
			m.Version = manifestVersion
		}
		mod := ModMetadata{Loader: loader, ModID: m.ModID, Name: m.Name, Version: m.Version, SourceEntry: sourceEntry, Provides: m.Provides}
		for _, dep := range raw.Dependencies[m.ModID] {
			kind := dep.Type
			if kind == "" {
				kind = "required"
				if dep.Mandatory != nil && !*dep.Mandatory {
					kind = "optional"
				}
			}
			mod.Dependencies = append(mod.Dependencies, Requirement{ModID: dep.ModID, Type: kind, Versions: []string{dep.VersionRange}, Side: strings.ToLower(dep.Side)})
		}
		mods = append(mods, mod)
	}
	return mods, nil
}

func parseFabricJSON(content string) (ModMetadata, bool) {
	var raw struct {
		ID          string                     `json:"id"`
		Name        string                     `json:"name"`
		Version     string                     `json:"version"`
		Environment string                     `json:"environment"`
		Provides    []string                   `json:"provides"`
		Depends     map[string]json.RawMessage `json:"depends"`
		Breaks      map[string]json.RawMessage `json:"breaks"`
		Conflicts   map[string]json.RawMessage `json:"conflicts"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil || raw.ID == "" {
		return ModMetadata{}, false
	}
	mod := ModMetadata{Loader: "fabric", ModID: raw.ID, Name: raw.Name, Version: raw.Version, SourceEntry: "fabric.mod.json", Environment: raw.Environment, Provides: raw.Provides}
	for _, relations := range []struct {
		kind    string
		entries map[string]json.RawMessage
	}{{"required", raw.Depends}, {"incompatible", raw.Breaks}, {"discouraged", raw.Conflicts}} {
		ids := make([]string, 0, len(relations.entries))
		for id := range relations.entries {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			data := relations.entries[id]
			var single string
			versions := []string{}
			if json.Unmarshal(data, &single) == nil {
				versions = append(versions, single)
			} else if json.Unmarshal(data, &versions) != nil {
				versions = []string{"<unsupported>"}
			}
			mod.Dependencies = append(mod.Dependencies, Requirement{ModID: id, Type: relations.kind, Versions: versions})
		}
	}
	return mod, true
}
