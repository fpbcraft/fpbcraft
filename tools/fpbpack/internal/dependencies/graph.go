package dependencies

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

type Finding struct {
	Code    string
	Level   string
	Mod     string
	Path    string
	Message string
}

type node struct {
	file inventory.ModFile
	mod  inventory.ModMetadata
}

// Analyze uses physical runtime boundaries. Server/common delivery to clients is
// deliberately uncertain until verified against an AutoModpack generation.
func Analyze(files []inventory.ModFile) []Finding {
	nodes := []node{}
	byID := map[string][]node{}
	for _, file := range files {
		for _, mod := range file.Metadata {
			if mod.ModID == "" {
				continue
			}
			n := node{file, mod}
			nodes = append(nodes, n)
			for _, id := range append([]string{mod.ModID}, mod.Provides...) {
				byID[id] = append(byID[id], n)
			}
		}
	}
	findings := []Finding{}
	for _, n := range nodes {
		for _, other := range byID[n.mod.ModID] {
			if other.file.Path <= n.file.Path || n.mod.Embedded || other.mod.Embedded {
				continue
			}
			if sameScope(n.file, other.file) {
				for _, affected := range []node{n, other} {
					findings = append(findings, Finding{"runtime_mod_duplicate", "blocking", affected.mod.ModID, affected.file.Path, fmt.Sprintf("%s is also supplied by %s; this runtime would load two JARs for the same mod ID.", affected.mod.ModID, oppositePath(affected, n, other))})
				}
			} else if !strings.EqualFold(n.file.SHA512, other.file.SHA512) && n.mod.Version != other.mod.Version {
				findings = append(findings, Finding{"runtime_version_mismatch", "warning", n.mod.ModID, n.file.Path, fmt.Sprintf("%s has version %s at %s and version %s at %s; review whether these placements should share a release.", n.mod.ModID, n.mod.Version, n.file.Path, other.mod.Version, other.file.Path)})
			}
		}
		for _, requirement := range n.mod.Dependencies {
			if requirement.Type != "required" && requirement.Type != "optional" && requirement.Type != "incompatible" && requirement.Type != "discouraged" {
				continue
			}
			if builtin(requirement.ModID) || !applies(requirement.Side, n.file.Location) {
				continue
			}
			visible := []node{}
			possible := false
			for _, supplied := range requirementProviders(byID, requirement.ModID, n) {
				if visibleTo(supplied.file, n.file) {
					visible = append(visible, supplied)
				} else if n.file.Location == inventory.LocationClient && supplied.file.Location == inventory.LocationServer {
					possible = true
				}
			}
			if len(visible) == 0 {
				if requirement.Type != "required" {
					continue
				}
				code, level, message := "required_mod_missing", "blocking", fmt.Sprintf("%s requires %s in %s.", n.mod.ModID, requirement.ModID, scope(n.file))
				if scopeMetadataIncomplete(files, n.file) {
					code, level, message = "dependency_metadata_unverified", "warning", fmt.Sprintf("%s requires %s, but its availability in %s is unverified because some JAR metadata is incomplete or runtime-selected.", n.mod.ModID, requirement.ModID, scope(n.file))
				} else if possible {
					code, level, message = "dependency_delivery_unverified", "warning", fmt.Sprintf("%s requires %s; only a server/common copy is known. Verify AutoModpack delivery to %s.", n.mod.ModID, requirement.ModID, scope(n.file))
				}
				findings = append(findings, Finding{code, level, n.mod.ModID, n.file.Path, message})
				continue
			}
			matched := false
			unknown := false
			allMatched := true
			hasEmbedded := false
			for _, supplied := range visible {
				ok, err := Matches(supplied.mod.Version, requirement.Versions, n.mod.Loader)
				matched = matched || ok
				allMatched = allMatched && ok
				hasEmbedded = hasEmbedded || supplied.mod.Embedded
				unknown = unknown || err != nil || supplied.file.Error != ""
			}
			if unknown || (hasEmbedded && matched && !allMatched) {
				findings = append(findings, Finding{"dependency_constraint_unverified", "warning", n.mod.ModID, n.file.Path, fmt.Sprintf("Cannot verify %s requirement %s %v with the installed version metadata.", n.mod.ModID, requirement.ModID, requirement.Versions)})
				continue
			}
			if (requirement.Type == "required" || requirement.Type == "optional") && !matched {
				findings = append(findings, Finding{"required_mod_version", "blocking", n.mod.ModID, n.file.Path, fmt.Sprintf("%s requires %s %v; the proposed runtime does not supply a matching version.", n.mod.ModID, requirement.ModID, requirement.Versions)})
			}
			if (requirement.Type == "incompatible" || requirement.Type == "discouraged") && matched {
				level := "blocking"
				if requirement.Type == "discouraged" {
					level = "warning"
				}
				findings = append(findings, Finding{"incompatible_mod_installed", level, n.mod.ModID, n.file.Path, fmt.Sprintf("%s declares %s %v %s.", n.mod.ModID, requirement.ModID, requirement.Versions, requirement.Type)})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		return a.Path+a.Code+a.Message < b.Path+b.Code+b.Message
	})
	return findings
}

func oppositePath(affected, a, b node) string {
	if affected.file.Path == a.file.Path {
		return b.file.Path
	}
	return a.file.Path
}
func scope(file inventory.ModFile) string {
	if file.Location == inventory.LocationServer {
		return "server"
	}
	group := file.Group
	if group == "" {
		group = "main"
	}
	return "AutoModpack/" + group
}
func sameScope(a, b inventory.ModFile) bool {
	if a.Location != b.Location {
		return false
	}
	if a.Location == inventory.LocationServer {
		return true
	}
	return a.Group == b.Group || a.Group == "" || a.Group == "main" || b.Group == "" || b.Group == "main"
}
func visibleTo(supplied, dependent inventory.ModFile) bool {
	if supplied.Location != dependent.Location {
		return false
	}
	if dependent.Location == inventory.LocationServer {
		return true
	}
	return supplied.Group == "" || supplied.Group == "main" || supplied.Group == dependent.Group
}
func applies(side string, location inventory.Location) bool {
	return side == "" || strings.EqualFold(side, "both") || strings.EqualFold(side, string(location))
}
func builtin(id string) bool {
	switch id {
	case "minecraft", "neoforge", "forge", "java", "fabricloader":
		return true
	}
	return false
}
