package dependencies

import (
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"strings"
)

func requirementProviders(byID map[string][]node, id string, dependent node) []node {
	candidates := append([]node(nil), byID[id]...)
	// Connector reverses NeoForge's underscore IDs for Fabric dependency lookup.
	// Keep this contextual: ordinary NeoForge IDs are not interchangeable.
	if dependent.mod.Loader == "fabric" && strings.Contains(id, "-") {
		for _, connector := range byID["connector"] {
			if visibleTo(connector.file, dependent.file) {
				candidates = append(candidates, byID[strings.ReplaceAll(id, "-", "_")]...)
				break
			}
		}
	}
	result := []node{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		key := candidate.file.Path + "\x00" + candidate.mod.SourceEntry + "\x00" + candidate.mod.ModID
		if !seen[key] {
			result = append(result, candidate)
			seen[key] = true
		}
	}
	return result
}

func scopeMetadataIncomplete(files []inventory.ModFile, dependent inventory.ModFile) bool {
	for _, file := range files {
		if file.Error != "" && (visibleTo(file, dependent) || (dependent.Location == inventory.LocationClient && file.Location == inventory.LocationServer)) {
			return true
		}
	}
	return false
}
