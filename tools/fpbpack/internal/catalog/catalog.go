package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

const ReportSchemaVersion = 1

type Options struct {
	Name       string
	Author     string
	Version    string
	Minecraft  string
	NeoForge   string
	OutputPath string
	Force      bool
}

type Source struct {
	Location inventory.Location `json:"location"`
	Path     string             `json:"path"`
}

type Duplicate struct {
	SHA512 string   `json:"sha512"`
	Files  []Source `json:"files"`
}

type Unresolved struct {
	SHA512   string                  `json:"sha512"`
	Filename string                  `json:"filename"`
	Sources  []Source                `json:"sources"`
	Metadata []inventory.ModMetadata `json:"metadata,omitempty"`
}

type ConflictFile struct {
	VersionID     string   `json:"version_id"`
	VersionNumber string   `json:"version_number"`
	Filename      string   `json:"filename"`
	SHA512        string   `json:"sha512"`
	Sources       []Source `json:"sources"`
}

type Conflict struct {
	ProjectID string         `json:"project_id"`
	Files     []ConflictFile `json:"files"`
}

type PlacementWarning struct {
	ProjectID   string             `json:"project_id"`
	Environment string             `json:"environment"`
	Deployment  inventory.Location `json:"deployment"`
	Filename    string             `json:"filename"`
}

type Summary struct {
	InventoryJARs      int `json:"inventory_jars"`
	UniqueArtifacts    int `json:"unique_artifacts"`
	DuplicateArtifacts int `json:"duplicate_artifacts"`
	GeneratedProjects  int `json:"generated_projects"`
	Unresolved         int `json:"unresolved"`
	ConflictProjects   int `json:"conflict_projects"`
	PlacementWarnings  int `json:"placement_warnings"`
}

type Report struct {
	SchemaVersion   int                `json:"schema_version"`
	InventorySchema int                `json:"inventory_schema_version"`
	Summary         Summary            `json:"summary"`
	Managed         []Entry            `json:"managed"`
	Duplicates      []Duplicate        `json:"duplicates,omitempty"`
	Unresolved      []Unresolved       `json:"unresolved,omitempty"`
	Conflicts       []Conflict         `json:"conflicts,omitempty"`
	Placement       []PlacementWarning `json:"placement_warnings,omitempty"`
}

type Entry struct {
	ProjectID   string             `json:"project_id"`
	VersionID   string             `json:"version_id"`
	Name        string             `json:"name"`
	Filename    string             `json:"filename"`
	SHA512      string             `json:"sha512"`
	URL         string             `json:"url"`
	Side        string             `json:"side"`
	Deployment  inventory.Location `json:"deployment"`
	Environment string             `json:"environment,omitempty"`
	SourcePaths []Source           `json:"source_paths"`
}

type Result struct {
	Entries []Entry
	Report  Report
}

type artifact struct {
	canonical inventory.ModFile
	sources   []Source
}

type indexFile struct {
	path string
	hash string
}

func Build(inv inventory.Inventory) (Result, error) {
	if inv.SchemaVersion != inventory.SchemaVersion {
		return Result{}, fmt.Errorf("unsupported inventory schema %d (expected %d)", inv.SchemaVersion, inventory.SchemaVersion)
	}
	if !inv.ModrinthChecked {
		return Result{}, fmt.Errorf("inventory has no completed Modrinth exact-hash lookup")
	}

	groups := map[string][]inventory.ModFile{}
	for _, mod := range inv.Mods {
		key := mod.SHA512
		if key == "" {
			key = "path:" + mod.Path
		}
		groups[key] = append(groups[key], mod)
	}

	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	artifacts := make([]artifact, 0, len(keys))
	report := Report{SchemaVersion: ReportSchemaVersion, InventorySchema: inv.SchemaVersion}
	report.Summary.InventoryJARs = len(inv.Mods)
	report.Summary.UniqueArtifacts = len(keys)

	for _, key := range keys {
		members := append([]inventory.ModFile(nil), groups[key]...)
		sort.Slice(members, func(i, j int) bool {
			if members[i].Location != members[j].Location {
				return members[i].Location == inventory.LocationServer
			}
			return members[i].Path < members[j].Path
		})
		sources := make([]Source, 0, len(members))
		for _, member := range members {
			sources = append(sources, Source{Location: member.Location, Path: member.Path})
		}
		if len(members) > 1 {
			report.Duplicates = append(report.Duplicates, Duplicate{SHA512: members[0].SHA512, Files: sources})
		}
		artifacts = append(artifacts, artifact{canonical: members[0], sources: sources})
	}
	report.Summary.DuplicateArtifacts = len(report.Duplicates)

	byProject := map[string][]artifact{}
	for _, item := range artifacts {
		if item.canonical.Modrinth == nil {
			report.Unresolved = append(report.Unresolved, Unresolved{
				SHA512: item.canonical.SHA512, Filename: item.canonical.Filename,
				Sources: item.sources, Metadata: item.canonical.Metadata,
			})
			continue
		}
		byProject[item.canonical.Modrinth.ProjectID] = append(byProject[item.canonical.Modrinth.ProjectID], item)
	}

	projectIDs := make([]string, 0, len(byProject))
	for projectID := range byProject {
		projectIDs = append(projectIDs, projectID)
	}
	sort.Strings(projectIDs)

	entries := make([]Entry, 0, len(projectIDs))
	for _, projectID := range projectIDs {
		items := byProject[projectID]
		if len(items) != 1 {
			conflict := Conflict{ProjectID: projectID}
			for _, item := range items {
				match := item.canonical.Modrinth
				conflict.Files = append(conflict.Files, ConflictFile{
					VersionID: match.VersionID, VersionNumber: match.VersionNumber,
					Filename: item.canonical.Filename, SHA512: item.canonical.SHA512, Sources: item.sources,
				})
			}
			sort.Slice(conflict.Files, func(i, j int) bool { return conflict.Files[i].Filename < conflict.Files[j].Filename })
			report.Conflicts = append(report.Conflicts, conflict)
			continue
		}

		item := items[0]
		match := item.canonical.Modrinth
		entry := Entry{
			ProjectID: projectID, VersionID: match.VersionID,
			Name: displayName(item.canonical), Filename: item.canonical.Filename,
			SHA512: item.canonical.SHA512, URL: match.URL, Side: packwizSide(match.Environment, item.canonical.Location),
			Deployment: item.canonical.Location, Environment: match.Environment, SourcePaths: item.sources,
		}
		if placementMismatch(entry.Deployment, entry.Environment) {
			report.Placement = append(report.Placement, PlacementWarning{
				ProjectID: projectID, Environment: entry.Environment, Deployment: entry.Deployment, Filename: entry.Filename,
			})
		}
		if entry.URL == "" || entry.SHA512 == "" {
			report.Unresolved = append(report.Unresolved, Unresolved{
				SHA512: item.canonical.SHA512, Filename: item.canonical.Filename,
				Sources: item.sources, Metadata: item.canonical.Metadata,
			})
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ProjectID < entries[j].ProjectID })

	report.Managed = append([]Entry(nil), entries...)
	report.Summary.GeneratedProjects = len(entries)
	report.Summary.Unresolved = len(report.Unresolved)
	report.Summary.ConflictProjects = len(report.Conflicts)
	report.Summary.PlacementWarnings = len(report.Placement)
	sort.Slice(report.Unresolved, func(i, j int) bool { return report.Unresolved[i].Filename < report.Unresolved[j].Filename })
	sort.Slice(report.Placement, func(i, j int) bool { return report.Placement[i].Filename < report.Placement[j].Filename })
	return Result{Entries: entries, Report: report}, nil
}

func Write(result Result, opts Options) error {
	if opts.OutputPath == "" {
		return fmt.Errorf("output path is required")
	}
	if opts.Name == "" {
		opts.Name = "FPBCraft"
	}
	if opts.Author == "" {
		opts.Author = "FPBCraft"
	}
	if opts.Version == "" {
		opts.Version = "migration"
	}
	if opts.Minecraft == "" {
		opts.Minecraft = "1.21.1"
	}

	if info, err := os.Stat(opts.OutputPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("output exists and is not a directory: %s", opts.OutputPath)
		}
		entries, readErr := os.ReadDir(opts.OutputPath)
		if readErr != nil {
			return readErr
		}
		if len(entries) > 0 && !opts.Force {
			return fmt.Errorf("output directory is not empty; use --force to replace it: %s", opts.OutputPath)
		}
		if opts.Force {
			if err := os.RemoveAll(opts.OutputPath); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	tmp, err := os.MkdirTemp(filepath.Dir(filepath.Clean(opts.OutputPath)), ".fpbpack-catalog-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	modsDir := filepath.Join(tmp, "mods")
	if err := os.MkdirAll(modsDir, 0o755); err != nil {
		return err
	}

	indexFiles := make([]indexFile, 0, len(result.Entries))
	for _, entry := range result.Entries {
		content := renderMetafile(entry)
		rel := filepath.ToSlash(filepath.Join("mods", entry.ProjectID+".pw.toml"))
		path := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		indexFiles = append(indexFiles, indexFile{path: rel, hash: sha256Hex([]byte(content))})
	}
	indexContent := renderIndex(indexFiles)
	if err := os.WriteFile(filepath.Join(tmp, "index.toml"), []byte(indexContent), 0o644); err != nil {
		return err
	}
	packContent := renderPack(opts, sha256Hex([]byte(indexContent)))
	if err := os.WriteFile(filepath.Join(tmp, "pack.toml"), []byte(packContent), 0o644); err != nil {
		return err
	}
	reportBytes, err := json.MarshalIndent(result.Report, "", "  ")
	if err != nil {
		return err
	}
	reportBytes = append(reportBytes, '\n')
	if err := os.WriteFile(filepath.Join(tmp, "migration-report.json"), reportBytes, 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmp, opts.OutputPath); err != nil {
		return err
	}
	return nil
}

func displayName(mod inventory.ModFile) string {
	if mod.Modrinth != nil && mod.Modrinth.VersionName != "" {
		return mod.Modrinth.VersionName
	}
	return strings.TrimSuffix(mod.Filename, filepath.Ext(mod.Filename))
}

func packwizSide(environment string, deployment inventory.Location) string {
	switch environment {
	case "client_only", "client_only_server_optional":
		return "client"
	case "server_only", "server_only_client_optional":
		return "server"
	case "client_and_server", "client_or_server", "client_or_server_prefers_both":
		return "both"
	}
	if deployment == inventory.LocationClient {
		return "client"
	}
	return "both"
}

func placementMismatch(deployment inventory.Location, environment string) bool {
	if deployment == inventory.LocationClient {
		return environment == "server_only" || environment == "server_only_client_optional"
	}
	return environment == "client_only" || environment == "client_only_server_optional"
}

func renderMetafile(entry Entry) string {
	return fmt.Sprintf("name = %s\nfilename = %s\nside = %s\n\n[download]\nhash-format = \"sha512\"\nhash = %s\nmode = \"url\"\nurl = %s\n\n[update.modrinth]\nmod-id = %s\nversion = %s\n",
		strconv.Quote(entry.Name), strconv.Quote(entry.Filename), strconv.Quote(entry.Side), strconv.Quote(entry.SHA512), strconv.Quote(entry.URL), strconv.Quote(entry.ProjectID), strconv.Quote(entry.VersionID))
}

func renderIndex(files []indexFile) string {
	var b strings.Builder
	b.WriteString("hash-format = \"sha256\"\n")
	for _, file := range files {
		b.WriteString("\n[[files]]\n")
		b.WriteString("file = " + strconv.Quote(file.path) + "\n")
		b.WriteString("hash = " + strconv.Quote(file.hash) + "\n")
		b.WriteString("metafile = true\n")
	}
	return b.String()
}

func renderPack(opts Options, indexHash string) string {
	var b strings.Builder
	b.WriteString("name = " + strconv.Quote(opts.Name) + "\n")
	b.WriteString("author = " + strconv.Quote(opts.Author) + "\n")
	b.WriteString("version = " + strconv.Quote(opts.Version) + "\n")
	b.WriteString("pack-format = \"packwiz:1.1.0\"\n\n")
	b.WriteString("[index]\nfile = \"index.toml\"\nhash-format = \"sha256\"\nhash = " + strconv.Quote(indexHash) + "\n\n")
	b.WriteString("[versions]\nminecraft = " + strconv.Quote(opts.Minecraft) + "\n")
	if opts.NeoForge != "" {
		b.WriteString("neoforge = " + strconv.Quote(opts.NeoForge) + "\n")
	}
	return b.String()
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
