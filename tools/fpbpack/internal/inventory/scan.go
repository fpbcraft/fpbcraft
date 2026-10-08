package inventory

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DefaultServerModsPath = "mods"
	DefaultClientModsPath = "automodpack/host-modpack/main/mods"
	DefaultAutoModpackHostPath = "automodpack/host-modpack"
)

type ScanOptions struct {
	ServerRoot          string
	ServerModsPath      string
	ClientModsPath      string
	AutoModpackHostPath string
	// OnFile reports each inspected artifact; callers may use it to display live progress.
	OnFile func(ModFile)
}

func Scan(options ScanOptions) (Inventory, error) {
	if options.ServerRoot == "" {
		return Inventory{}, fmt.Errorf("server root is required")
	}
	if options.ServerModsPath == "" {
		options.ServerModsPath = DefaultServerModsPath
	}
	if options.ClientModsPath == "" {
		options.ClientModsPath = DefaultClientModsPath
	}
	if options.AutoModpackHostPath == "" {
		options.AutoModpackHostPath = DefaultAutoModpackHostPath
	}

	root, err := filepath.Abs(options.ServerRoot)
	if err != nil {
		return Inventory{}, fmt.Errorf("resolve server root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return Inventory{}, fmt.Errorf("server root: %w", err)
	}
	if !info.IsDir() {
		return Inventory{}, fmt.Errorf("server root is not a directory: %s", root)
	}

	result := Inventory{
		SchemaVersion:  SchemaVersion,
		GeneratedAt:    time.Now().UTC(),
		ServerRoot:     root,
		ServerModsPath: options.ServerModsPath,
		ClientModsPath: options.ClientModsPath,
		ClientGroupModsPaths: map[string]string{},
	}

	type scanTarget struct {
		location Location
		group    string
		relative string
	}
	targets := []scanTarget{{location: LocationServer, relative: options.ServerModsPath}}
	seenClientPaths := map[string]struct{}{}
	addClientTarget := func(group, relative string) {
		relative = filepath.ToSlash(filepath.Clean(relative))
		if relative == "." || relative == "" {
			return
		}
		key := strings.ToLower(relative)
		if _, exists := seenClientPaths[key]; exists {
			return
		}
		seenClientPaths[key] = struct{}{}
		if strings.TrimSpace(group) == "" {
			group = "main"
		}
		result.ClientGroupModsPaths[group] = relative
		targets = append(targets, scanTarget{location: LocationClient, group: group, relative: relative})
	}

	configuredGroup := clientGroupFromModsPath(options.ClientModsPath, options.AutoModpackHostPath)
	addClientTarget(configuredGroup, options.ClientModsPath)
	hostRoot := filepath.Join(root, filepath.FromSlash(options.AutoModpackHostPath))
	if entries, readErr := os.ReadDir(hostRoot); readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			group := strings.TrimSpace(entry.Name())
			if group == "" {
				continue
			}
			addClientTarget(group, filepath.ToSlash(filepath.Join(options.AutoModpackHostPath, group, "mods")))
		}
	} else if !os.IsNotExist(readErr) {
		return Inventory{}, fmt.Errorf("read AutoModpack group directory %s: %w", hostRoot, readErr)
	}

	foundDirectory := false
	for _, target := range targets {
		dir := filepath.Join(root, filepath.FromSlash(target.relative))
		mods, exists, err := scanDirectory(root, dir, target.location, target.group, options.OnFile)
		if err != nil {
			return Inventory{}, err
		}
		if !exists {
			if target.location == LocationServer || target.relative == filepath.ToSlash(filepath.Clean(options.ClientModsPath)) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s mod directory does not exist: %s", target.location, dir))
			}
			continue
		}
		foundDirectory = true
		result.Mods = append(result.Mods, mods...)
	}

	if !foundDirectory {
		return Inventory{}, fmt.Errorf("neither configured mod directory exists under %s", root)
	}

	sort.Slice(result.Mods, func(a, b int) bool {
		if result.Mods[a].Location != result.Mods[b].Location {
			return result.Mods[a].Location < result.Mods[b].Location
		}
		if result.Mods[a].Group != result.Mods[b].Group {
			return strings.ToLower(result.Mods[a].Group) < strings.ToLower(result.Mods[b].Group)
		}
		return strings.ToLower(result.Mods[a].Filename) < strings.ToLower(result.Mods[b].Filename)
	})
	result.RecalculateSummary()
	return result, nil
}

func scanDirectory(root, dir string, location Location, group string, onFile func(ModFile)) ([]ModFile, bool, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s mod directory %s: %w", location, dir, err)
	}

	mods := make([]ModFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".jar") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		mod, err := inspectFile(root, path, location, group)
		if err != nil {
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				relative = path
			}
			mod := ModFile{
				Location: location,
				Group:    group,
				Path:     filepath.ToSlash(relative),
				Filename: entry.Name(),
				Error:    err.Error(),
			}
			mods = append(mods, mod)
			if onFile != nil {
				onFile(mod)
			}
			continue
		}
		mods = append(mods, mod)
		if onFile != nil {
			onFile(mod)
		}
	}
	return mods, true, nil
}

func inspectFile(root, path string, location Location, group string) (ModFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return ModFile{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return ModFile{}, fmt.Errorf("stat %s: %w", path, err)
	}

	sha1Hash := sha1.New()
	sha512Hash := sha512.New()
	normalizedLength := &normalizedLengthWriter{}
	if _, err := io.Copy(io.MultiWriter(sha1Hash, sha512Hash, normalizedLength), file); err != nil {
		return ModFile{}, fmt.Errorf("hash %s: %w", path, err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return ModFile{}, fmt.Errorf("rewind %s for CurseForge fingerprint: %w", path, err)
	}
	curseForgeFingerprint, err := computeCurseForgeFingerprint(file, normalizedLength.n)
	if err != nil {
		return ModFile{}, fmt.Errorf("CurseForge fingerprint %s: %w", path, err)
	}

	relative, err := filepath.Rel(root, path)
	if err != nil {
		return ModFile{}, fmt.Errorf("make path relative to server root: %w", err)
	}
	mod := ModFile{
		Location:              location,
		Group:                 group,
		Path:                  filepath.ToSlash(relative),
		Filename:              filepath.Base(path),
		Size:                  info.Size(),
		SHA1:                  hex.EncodeToString(sha1Hash.Sum(nil)),
		SHA512:                hex.EncodeToString(sha512Hash.Sum(nil)),
		CurseForgeFingerprint: curseForgeFingerprint,
	}

	metadata, err := ReadMetadata(path)
	if err != nil {
		mod.Error = err.Error()
	} else {
		mod.Metadata = metadata
	}
	return mod, nil
}


func clientGroupFromModsPath(clientModsPath, hostPath string) string {
	clientModsPath = filepath.ToSlash(filepath.Clean(clientModsPath))
	hostPath = strings.TrimSuffix(filepath.ToSlash(filepath.Clean(hostPath)), "/")
	prefix := hostPath + "/"
	if strings.HasPrefix(clientModsPath, prefix) {
		rest := strings.TrimPrefix(clientModsPath, prefix)
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] == "mods" {
			return parts[0]
		}
	}
	return "main"
}
