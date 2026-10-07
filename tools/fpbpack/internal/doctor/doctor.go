package doctor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

type Level string

const (
	LevelInfo     Level = "info"
	LevelWarning  Level = "warning"
	LevelBlocking Level = "blocking"
)

type Finding struct {
	Code       string `json:"code"`
	Level      Level  `json:"level"`
	Actionable bool   `json:"actionable"`
	Message    string `json:"message"`
	Mod        string `json:"mod,omitempty"`
	Path       string `json:"path,omitempty"`
}

type Summary struct {
	Blocking   int `json:"blocking"`
	Warnings   int `json:"warnings"`
	Info       int `json:"info"`
	Actionable int `json:"actionable"`
}

type Report struct {
	Summary  Summary   `json:"summary"`
	Findings []Finding `json:"findings"`
}

func Analyze(inv inventory.Inventory, cat catalog.Report) Report {
	findings := make([]Finding, 0)
	add := func(f Finding) {
		findings = append(findings, f)
	}

	if cat.InventorySchema != 0 && cat.InventorySchema != inv.SchemaVersion {
		add(Finding{
			Code:       "inventory_schema_mismatch",
			Level:      LevelBlocking,
			Actionable: true,
			Message:    fmt.Sprintf("catalog was generated from inventory schema %d but current inventory uses schema %d", cat.InventorySchema, inv.SchemaVersion),
		})
	}

	if inv.ModrinthError != "" {
		add(Finding{
			Code:       "modrinth_lookup_failed",
			Level:      LevelWarning,
			Actionable: true,
			Message:    "Modrinth matching is incomplete: " + inv.ModrinthError,
		})
	}
	if inv.CurseForgeError != "" {
		add(Finding{
			Code:       "curseforge_lookup_failed",
			Level:      LevelWarning,
			Actionable: true,
			Message:    "CurseForge matching is incomplete: " + inv.CurseForgeError,
		})
	}

	for _, mod := range inv.Mods {
		if strings.TrimSpace(mod.Error) == "" {
			continue
		}
		add(Finding{
			Code:       "metadata_unreadable",
			Level:      LevelWarning,
			Actionable: true,
			Message:    mod.Error,
			Mod:        displayModName(mod),
			Path:       mod.Path,
		})
	}

	for _, item := range cat.Unresolved {
		add(Finding{
			Code:       "unresolved_artifact",
			Level:      LevelBlocking,
			Actionable: true,
			Message:    "artifact has no verified management source",
			Mod:        item.Filename,
			Path:       firstSourcePath(item.Sources),
		})
	}
	for _, conflict := range cat.Conflicts {
		add(Finding{
			Code:       "project_version_conflict",
			Level:      LevelBlocking,
			Actionable: true,
			Message:    fmt.Sprintf("%s project %s has %d installed artifacts", conflict.Provider, conflict.ProjectID, len(conflict.Files)),
			Mod:        conflict.ProjectID,
		})
	}
	for _, warning := range cat.Placement {
		add(Finding{
			Code:       "placement_warning",
			Level:      LevelWarning,
			Actionable: true,
			Message:    fmt.Sprintf("provider environment %q disagrees with deployment %q", warning.Environment, warning.Deployment),
			Mod:        warning.Filename,
		})
	}
	if len(cat.Pinned) > 0 {
		add(Finding{
			Code:       "unmanaged_artifacts",
			Level:      LevelInfo,
			Actionable: false,
			Message:    fmt.Sprintf("%d artifact(s) are explicitly unmanaged and excluded from update actions", len(cat.Pinned)),
		})
	}

	currentByPath := make(map[string]inventory.ModFile, len(inv.Mods))
	currentByHash := make(map[string][]inventory.ModFile, len(inv.Mods))
	for _, mod := range inv.Mods {
		currentByPath[pathKey(mod.Location, mod.Path)] = mod
		if mod.SHA512 != "" {
			key := strings.ToLower(mod.SHA512)
			currentByHash[key] = append(currentByHash[key], mod)
		}
	}

	managedPaths := map[string]struct{}{}
	movedManagedPaths := map[string]struct{}{}
	knownOtherPaths := map[string]struct{}{}

	for _, entry := range cat.Managed {
		for _, source := range entry.SourcePaths {
			key := pathKey(source.Location, source.Path)
			managedPaths[key] = struct{}{}
			current, exists := currentByPath[key]
			if !exists {
				if candidates := currentByHash[strings.ToLower(entry.SHA512)]; len(candidates) > 0 {
					for _, candidate := range candidates {
						movedManagedPaths[pathKey(candidate.Location, candidate.Path)] = struct{}{}
					}
					add(Finding{
						Code:       "managed_artifact_moved",
						Level:      LevelBlocking,
						Actionable: true,
						Message:    fmt.Sprintf("managed artifact expected at %s but matching bytes now exist at %s", source.Path, candidates[0].Path),
						Mod:        entry.Name,
						Path:       source.Path,
					})
				} else {
					add(Finding{
						Code:       "managed_artifact_missing",
						Level:      LevelBlocking,
						Actionable: true,
						Message:    "managed artifact is missing from its accepted deployment path",
						Mod:        entry.Name,
						Path:       source.Path,
					})
				}
				continue
			}
			if entry.SHA512 != "" && !strings.EqualFold(current.SHA512, entry.SHA512) {
				add(Finding{
					Code:       "managed_artifact_replaced",
					Level:      LevelBlocking,
					Actionable: true,
					Message:    "managed artifact bytes differ from the accepted catalog state",
					Mod:        entry.Name,
					Path:       source.Path,
				})
			}
		}
	}

	for _, item := range cat.Pinned {
		for _, source := range item.Sources {
			knownOtherPaths[pathKey(source.Location, source.Path)] = struct{}{}
		}
	}
	for _, item := range cat.Unresolved {
		for _, source := range item.Sources {
			knownOtherPaths[pathKey(source.Location, source.Path)] = struct{}{}
		}
	}
	for _, conflict := range cat.Conflicts {
		for _, file := range conflict.Files {
			for _, source := range file.Sources {
				knownOtherPaths[pathKey(source.Location, source.Path)] = struct{}{}
			}
		}
	}

	for _, mod := range inv.Mods {
		key := pathKey(mod.Location, mod.Path)
		if _, ok := managedPaths[key]; ok {
			continue
		}
		if _, ok := knownOtherPaths[key]; ok {
			continue
		}
		if _, ok := movedManagedPaths[key]; ok {
			continue
		}
		add(Finding{
			Code:       "external_artifact_added",
			Level:      LevelBlocking,
			Actionable: true,
			Message:    "artifact exists in a managed deployment directory but is not present in the accepted catalog state",
			Mod:        displayModName(mod),
			Path:       mod.Path,
		})
	}

	sort.SliceStable(findings, func(i, j int) bool {
		iRank := levelRank(findings[i].Level)
		jRank := levelRank(findings[j].Level)
		if iRank != jRank {
			return iRank < jRank
		}
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		if findings[i].Mod != findings[j].Mod {
			return findings[i].Mod < findings[j].Mod
		}
		return findings[i].Path < findings[j].Path
	})

	var summary Summary
	for _, finding := range findings {
		switch finding.Level {
		case LevelBlocking:
			summary.Blocking++
		case LevelWarning:
			summary.Warnings++
		default:
			summary.Info++
		}
		if finding.Actionable {
			summary.Actionable++
		}
	}

	return Report{Summary: summary, Findings: findings}
}

func pathKey(location inventory.Location, path string) string {
	return string(location) + ":" + strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "./")
}

func firstSourcePath(sources []catalog.Source) string {
	if len(sources) == 0 {
		return ""
	}
	return sources[0].Path
}

func displayModName(mod inventory.ModFile) string {
	for _, meta := range mod.Metadata {
		if strings.TrimSpace(meta.Name) != "" {
			return meta.Name
		}
	}
	return mod.Filename
}

func levelRank(level Level) int {
	switch level {
	case LevelBlocking:
		return 0
	case LevelWarning:
		return 1
	default:
		return 2
	}
}
