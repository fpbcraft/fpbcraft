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
)

type ScanOptions struct {
	ServerRoot     string
	ServerModsPath string
	ClientModsPath string
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
	}

	targets := []struct {
		location Location
		relative string
	}{
		{LocationServer, options.ServerModsPath},
		{LocationClient, options.ClientModsPath},
	}

	foundDirectory := false
	for _, target := range targets {
		dir := filepath.Join(root, filepath.FromSlash(target.relative))
		mods, exists, err := scanDirectory(root, dir, target.location)
		if err != nil {
			return Inventory{}, err
		}
		if !exists {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s mod directory does not exist: %s", target.location, dir))
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
		return strings.ToLower(result.Mods[a].Filename) < strings.ToLower(result.Mods[b].Filename)
	})
	result.RecalculateSummary()
	return result, nil
}

func scanDirectory(root, dir string, location Location) ([]ModFile, bool, error) {
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
		mod, err := inspectFile(root, path, location)
		if err != nil {
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				relative = path
			}
			mods = append(mods, ModFile{
				Location: location,
				Path:     filepath.ToSlash(relative),
				Filename: entry.Name(),
				Error:    err.Error(),
			})
			continue
		}
		mods = append(mods, mod)
	}
	return mods, true, nil
}

func inspectFile(root, path string, location Location) (ModFile, error) {
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
	if _, err := io.Copy(io.MultiWriter(sha1Hash, sha512Hash), file); err != nil {
		return ModFile{}, fmt.Errorf("hash %s: %w", path, err)
	}

	relative, err := filepath.Rel(root, path)
	if err != nil {
		return ModFile{}, fmt.Errorf("make path relative to server root: %w", err)
	}
	mod := ModFile{
		Location: location,
		Path:     filepath.ToSlash(relative),
		Filename: filepath.Base(path),
		Size:     info.Size(),
		SHA1:     hex.EncodeToString(sha1Hash.Sum(nil)),
		SHA512:   hex.EncodeToString(sha512Hash.Sum(nil)),
	}

	metadata, err := ReadMetadata(path)
	if err != nil {
		mod.Error = err.Error()
	} else {
		mod.Metadata = metadata
	}
	return mod, nil
}
