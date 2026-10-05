package catalog

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
)

type CurseForgeDetectSummary struct {
	Detected  int
	Unmatched int
	Output    string
}

type packwizCurseForgeMeta struct {
	Name       string
	Filename   string
	Side       string
	HashFormat string
	Hash       string
	ProjectID  uint32
	FileID     uint32
	Path       string
}

func ResolveCurseForgeWithPackwiz(inv inventory.Inventory, result *Result, outputPath, packwizPath string) (CurseForgeDetectSummary, error) {
	var summary CurseForgeDetectSummary
	if len(result.Report.Unresolved) == 0 {
		return summary, nil
	}
	executable, err := resolveExecutable(packwizPath)
	if err != nil {
		return summary, err
	}

	modsDir := filepath.Join(outputPath, "mods")
	if err := os.MkdirAll(modsDir, 0o755); err != nil {
		return summary, err
	}

	before, err := metafileHashes(modsDir)
	if err != nil {
		return summary, err
	}

	byFilename := make(map[string]Unresolved, len(result.Report.Unresolved))
	for _, unresolved := range result.Report.Unresolved {
		if len(unresolved.Sources) == 0 {
			return summary, fmt.Errorf("unresolved artifact %s has no source path", unresolved.Filename)
		}
		if _, exists := byFilename[unresolved.Filename]; exists {
			return summary, fmt.Errorf("cannot safely detect duplicate unresolved filename %q", unresolved.Filename)
		}
		source, err := inventorySourcePath(inv.ServerRoot, unresolved.Sources[0].Path)
		if err != nil {
			return summary, err
		}
		target := filepath.Join(modsDir, unresolved.Filename)
		if err := copyRegularFile(source, target); err != nil {
			return summary, fmt.Errorf("copy unresolved artifact %s: %w", unresolved.Filename, err)
		}
		byFilename[unresolved.Filename] = unresolved
	}

	detectOutput, err := runPackwiz(executable, outputPath, "curseforge", "detect", "--yes")
	summary.Output = detectOutput
	if err != nil {
		return summary, fmt.Errorf("packwiz curseforge detect failed: %w\n%s", err, strings.TrimSpace(detectOutput))
	}
	if !strings.Contains(detectOutput, "Detection complete!") {
		return summary, fmt.Errorf("packwiz curseforge detect did not complete successfully:\n%s", strings.TrimSpace(detectOutput))
	}

	matched := make(map[string]Unresolved)
	for filename, unresolved := range byFilename {
		path := filepath.Join(modsDir, filename)
		_, statErr := os.Stat(path)
		switch {
		case statErr == nil:
			summary.Unmatched++
			if err := os.Remove(path); err != nil {
				return summary, fmt.Errorf("remove unmatched temporary jar %s: %w", filename, err)
			}
		case os.IsNotExist(statErr):
			matched[filename] = unresolved
		default:
			return summary, statErr
		}
	}

	after, err := metafileHashes(modsDir)
	if err != nil {
		return summary, err
	}
	detected := make(map[string]packwizCurseForgeMeta)
	for path, hash := range after {
		if before[path] == hash {
			continue
		}
		meta, err := parsePackwizCurseForgeMeta(path)
		if err != nil {
			return summary, err
		}
		if meta.ProjectID == 0 || meta.FileID == 0 || meta.Filename == "" {
			continue
		}
		if _, ok := matched[meta.Filename]; !ok {
			continue
		}
		detected[meta.Filename] = meta
	}

	if len(detected) != len(matched) {
		missing := make([]string, 0)
		for filename := range matched {
			if _, ok := detected[filename]; !ok {
				missing = append(missing, filename)
			}
		}
		sort.Strings(missing)
		return summary, fmt.Errorf("packwiz removed %d matched jar(s) but produced usable metadata for %d; missing metadata for: %s", len(matched), len(detected), strings.Join(missing, ", "))
	}

	remaining := make([]Unresolved, 0, len(result.Report.Unresolved)-len(detected))
	for _, unresolved := range result.Report.Unresolved {
		meta, ok := detected[unresolved.Filename]
		if !ok {
			remaining = append(remaining, unresolved)
			continue
		}
		mod := findInventoryArtifact(inv, unresolved.SHA512, unresolved.Filename)
		deployment := unresolved.Sources[0].Location
		side := meta.Side
		if side == "" {
			side = packwizSide("", deployment)
		}
		entry := Entry{
			Provider:    "curseforge",
			ProjectID:   strconv.FormatUint(uint64(meta.ProjectID), 10),
			FileID:      meta.FileID,
			Name:        meta.Name,
			Filename:    meta.Filename,
			SHA1:        mod.SHA1,
			SHA512:      unresolved.SHA512,
			Side:        side,
			Deployment:  deployment,
			SourcePaths: unresolved.Sources,
		}
		result.Entries = append(result.Entries, entry)
		result.Report.Managed = append(result.Report.Managed, entry)
		summary.Detected++
	}
	result.Report.Unresolved = remaining
	sort.Slice(result.Entries, func(i, j int) bool {
		if result.Entries[i].Provider != result.Entries[j].Provider {
			return result.Entries[i].Provider < result.Entries[j].Provider
		}
		return result.Entries[i].ProjectID < result.Entries[j].ProjectID
	})
	sort.Slice(result.Report.Managed, func(i, j int) bool {
		if result.Report.Managed[i].Provider != result.Report.Managed[j].Provider {
			return result.Report.Managed[i].Provider < result.Report.Managed[j].Provider
		}
		return result.Report.Managed[i].ProjectID < result.Report.Managed[j].ProjectID
	})
	result.Report.Summary.GeneratedProjects = len(result.Report.Managed)
	result.Report.Summary.Unresolved = len(result.Report.Unresolved)

	refreshOutput, err := runPackwiz(executable, outputPath, "refresh")
	summary.Output = strings.TrimSpace(detectOutput + "\n" + refreshOutput)
	if err != nil {
		return summary, fmt.Errorf("packwiz refresh failed: %w\n%s", err, strings.TrimSpace(refreshOutput))
	}
	if err := rewriteMigrationReport(outputPath, result.Report); err != nil {
		return summary, err
	}
	return summary, nil
}

func resolveExecutable(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = "packwiz"
	}
	if strings.ContainsRune(path, os.PathSeparator) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("packwiz executable: %w", err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("packwiz executable is a directory: %s", abs)
		}
		return abs, nil
	}
	resolved, err := exec.LookPath(path)
	if err != nil {
		return "", fmt.Errorf("find packwiz executable %q: %w", path, err)
	}
	return resolved, nil
}

func inventorySourcePath(root, relative string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(rootAbs, filepath.FromSlash(relative))
	rel, err := filepath.Rel(rootAbs, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("inventory source escapes server root: %s", relative)
	}
	return path, nil
}

func copyRegularFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", source)
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func runPackwiz(executable, dir string, args ...string) (string, error) {
	cmd := exec.Command(executable, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func metafileHashes(root string) (map[string]string, error) {
	hashes := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".pw.toml") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		hashes[path] = hex.EncodeToString(sum[:])
		return nil
	})
	return hashes, err
}

func parsePackwizCurseForgeMeta(path string) (packwizCurseForgeMeta, error) {
	file, err := os.Open(path)
	if err != nil {
		return packwizCurseForgeMeta{}, err
	}
	defer file.Close()

	meta := packwizCurseForgeMeta{Path: path}
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		raw = strings.TrimSpace(raw)
		value := raw
		if strings.HasPrefix(raw, "\"") {
			if decoded, err := strconv.Unquote(raw); err == nil {
				value = decoded
			}
		}
		switch section {
		case "":
			switch key {
			case "name":
				meta.Name = value
			case "filename":
				meta.Filename = value
			case "side":
				meta.Side = value
			}
		case "download":
			switch key {
			case "hash-format":
				meta.HashFormat = value
			case "hash":
				meta.Hash = value
			}
		case "update.curseforge":
			switch key {
			case "project-id":
				parsed, err := strconv.ParseUint(raw, 10, 32)
				if err != nil {
					return meta, fmt.Errorf("parse project-id in %s: %w", path, err)
				}
				meta.ProjectID = uint32(parsed)
			case "file-id":
				parsed, err := strconv.ParseUint(raw, 10, 32)
				if err != nil {
					return meta, fmt.Errorf("parse file-id in %s: %w", path, err)
				}
				meta.FileID = uint32(parsed)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return meta, err
	}
	return meta, nil
}

func findInventoryArtifact(inv inventory.Inventory, sha512, filename string) inventory.ModFile {
	for _, mod := range inv.Mods {
		if sha512 != "" && mod.SHA512 == sha512 {
			return mod
		}
	}
	for _, mod := range inv.Mods {
		if mod.Filename == filename {
			return mod
		}
	}
	return inventory.ModFile{}
}

func rewriteMigrationReport(outputPath string, report Report) error {
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	return os.WriteFile(filepath.Join(outputPath, "migration-report.json"), bytes, 0o644)
}
