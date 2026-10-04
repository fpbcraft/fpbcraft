package inventory

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

const metadataReadLimit = 1 << 20 // 1 MiB per metadata file

var tomlStringValue = regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*=\s*"([^"]*)"\s*(?:#.*)?$`)
var tomlSingleQuotedValue = regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*=\s*'([^']*)'\s*(?:#.*)?$`)

func ReadMetadata(path string) ([]ModMetadata, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open jar metadata: %w", err)
	}
	defer reader.Close()

	entries := make(map[string]*zip.File, len(reader.File))
	for _, entry := range reader.File {
		entries[filepath.ToSlash(entry.Name)] = entry
	}

	manifestVersion := ""
	if manifest := entries["META-INF/MANIFEST.MF"]; manifest != nil {
		if content, err := readZipText(manifest); err == nil {
			manifestVersion = readManifestVersion(content)
		}
	}

	for _, candidate := range []struct {
		name   string
		loader string
	}{
		{"META-INF/neoforge.mods.toml", "neoforge"},
		{"META-INF/mods.toml", "forge"},
	} {
		if entry := entries[candidate.name]; entry != nil {
			content, err := readZipText(entry)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", candidate.name, err)
			}
			mods := parseModsTOML(content, candidate.loader, candidate.name, manifestVersion)
			if len(mods) > 0 {
				return mods, nil
			}
		}
	}

	if entry := entries["fabric.mod.json"]; entry != nil {
		content, err := readZipText(entry)
		if err != nil {
			return nil, fmt.Errorf("read fabric.mod.json: %w", err)
		}
		if mod, ok := parseFabricJSON(content); ok {
			return []ModMetadata{mod}, nil
		}
	}

	return nil, nil
}

func readZipText(entry *zip.File) (string, error) {
	reader, err := entry.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	limited := io.LimitReader(reader, metadataReadLimit+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(content) > metadataReadLimit {
		return "", fmt.Errorf("metadata entry exceeds %d bytes", metadataReadLimit)
	}
	return string(content), nil
}

func parseModsTOML(content, loader, sourceEntry, manifestVersion string) []ModMetadata {
	var mods []ModMetadata
	var current *ModMetadata

	flush := func() {
		if current == nil || current.ModID == "" {
			return
		}
		if current.Version == "${file.jarVersion}" && manifestVersion != "" {
			current.Version = manifestVersion
		}
		mods = append(mods, *current)
	}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[[mods]]" {
			flush()
			current = &ModMetadata{Loader: loader, SourceEntry: sourceEntry}
			continue
		}
		if current == nil {
			continue
		}
		match := tomlStringValue.FindStringSubmatch(line)
		if match == nil {
			match = tomlSingleQuotedValue.FindStringSubmatch(line)
		}
		if match == nil {
			continue
		}
		key, value := match[1], match[2]
		switch key {
		case "modId":
			current.ModID = value
		case "displayName":
			current.Name = value
		case "version":
			current.Version = value
		}
	}
	flush()
	return mods
}

func readManifestVersion(content string) string {
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if value, ok := strings.CutPrefix(line, "Implementation-Version:"); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseFabricJSON(content string) (ModMetadata, bool) {
	var raw struct {
		ID      string          `json:"id"`
		Name    string          `json:"name"`
		Version json.RawMessage `json:"version"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil || raw.ID == "" {
		return ModMetadata{}, false
	}
	version := ""
	_ = json.Unmarshal(raw.Version, &version)
	return ModMetadata{
		Loader:      "fabric",
		ModID:       raw.ID,
		Name:        raw.Name,
		Version:     version,
		SourceEntry: "fabric.mod.json",
	}, true
}
