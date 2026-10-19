package inventory

import (
	"archive/zip"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const metadataReadLimit = 1 << 20 // 1 MiB per metadata file

func ReadMetadata(path string) ([]ModMetadata, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open jar metadata: %w", err)
	}
	defer reader.Close()

	return readArchiveMetadata(reader.File, 0, &embeddedBudget{remaining: 128 << 20, count: 64})
}

func readArchiveMetadata(files []*zip.File, depth int, budget *embeddedBudget) ([]ModMetadata, error) {
	entries := make(map[string]*zip.File, len(files))
	for _, entry := range files {
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
			mods, err := parseModsTOML(content, candidate.loader, candidate.name, manifestVersion)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", candidate.name, err)
			}
			if len(mods) > 0 {
				return withEmbeddedMetadata(mods, entries, depth, budget)
			}
		}
	}

	if entry := entries["fabric.mod.json"]; entry != nil {
		content, err := readZipText(entry)
		if err != nil {
			return nil, fmt.Errorf("read fabric.mod.json: %w", err)
		}
		if mod, ok := parseFabricJSON(content); ok {
			return withEmbeddedMetadata([]ModMetadata{mod}, entries, depth, budget)
		}
		return nil, fmt.Errorf("invalid fabric.mod.json")
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

func readManifestVersion(content string) string {
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if value, ok := strings.CutPrefix(line, "Implementation-Version:"); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
