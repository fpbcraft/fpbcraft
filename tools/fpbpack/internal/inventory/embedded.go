package inventory

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Only declared loader JARs are inspected. Bound nesting and decompression;
// arbitrary archive entries must not be treated as runtime dependencies.
type embeddedBudget struct {
	remaining int64
	count     int
}

func withEmbeddedMetadata(mods []ModMetadata, entries map[string]*zip.File, depth int, budget *embeddedBudget) ([]ModMetadata, error) {
	paths := []string{}
	seen := map[string]bool{}
	for _, descriptor := range []string{"META-INF/jarjar/metadata.json", "fabric.mod.json"} {
		entry := entries[descriptor]
		if entry == nil {
			continue
		}
		content, err := readZipText(entry)
		if err != nil {
			return mods, err
		}
		var raw struct {
			Jars []struct {
				Path string `json:"path"`
				File string `json:"file"`
			} `json:"jars"`
		}
		if err := json.Unmarshal([]byte(content), &raw); err != nil {
			return mods, fmt.Errorf("parse embedded JAR descriptor: %w", err)
		}
		for _, jar := range raw.Jars {
			path := jar.Path
			if path == "" {
				path = jar.File
			}
			if path != "" && !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	if len(paths) == 0 {
		return mods, nil
	}
	if depth >= 8 || len(paths) > 256 {
		return mods, fmt.Errorf("embedded JAR metadata limit exceeded")
	}
	for _, path := range paths {
		if budget.count <= 0 || budget.remaining <= 0 {
			return mods, fmt.Errorf("embedded metadata budget exceeded")
		}
		budget.count--
		entry := entries[path]
		if entry == nil {
			return mods, fmt.Errorf("declared embedded JAR is missing: %s", path)
		}
		reader, err := entry.Open()
		if err != nil {
			return mods, err
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, min(64<<20, budget.remaining)+1))
		reader.Close()
		if readErr != nil {
			return mods, readErr
		}
		budget.remaining -= int64(len(content))
		if len(content) > 64<<20 || budget.remaining < 0 {
			return mods, fmt.Errorf("embedded JAR exceeds 64 MiB: %s", path)
		}
		jar, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			return mods, err
		}
		embedded, err := readArchiveMetadata(jar.File, depth+1, budget)
		for i := range embedded {
			embedded[i].Embedded = true
			embedded[i].SourceEntry = path + "!" + embedded[i].SourceEntry
		}
		mods = append(mods, embedded...)
		if err != nil {
			return mods, err
		}
	}
	return mods, nil
}
