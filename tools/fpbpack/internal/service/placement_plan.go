package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func (s *Service) CreatePlacementPlan(ctx context.Context, path string) (planning.Plan, error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	path = normalizeCatalogPath(path)
	if path == "" {
		return planning.Plan{}, fmt.Errorf("mod path is required")
	}
	mod, ok := s.liveModByPath(path)
	if !ok {
		return planning.Plan{}, fmt.Errorf("live mod %q was not found", path)
	}
	entry, ok := s.managedEntryByPath(path)
	if !ok {
		return planning.Plan{}, fmt.Errorf("placement changes require a verified managed source")
	}
	if entry.Deployment == mod.Location {
		return planning.Plan{}, fmt.Errorf("current placement already matches the preferred placement")
	}

	currentRel, err := safeRelativePath(mod.Path)
	if err != nil {
		return planning.Plan{}, err
	}
	currentPath := filepath.Join(s.options.ServerRoot, currentRel)
	actual, err := sha512File(currentPath)
	if err != nil {
		return planning.Plan{}, fmt.Errorf("hash current artifact: %w", err)
	}
	if entry.SHA512 != "" && !strings.EqualFold(actual, entry.SHA512) {
		return planning.Plan{}, fmt.Errorf("current artifact differs from the accepted managed state")
	}

	targetDir := s.options.ServerModsPath
	if strings.TrimSpace(targetDir) == "" {
		targetDir = inventory.DefaultServerModsPath
	}
	if entry.Deployment == inventory.LocationClient {
		targetDir = s.options.ClientModsPath
		if strings.TrimSpace(targetDir) == "" {
			targetDir = inventory.DefaultClientModsPath
		}
	}
	targetPath := filepath.ToSlash(filepath.Join(targetDir, mod.Filename))
	targetRel, err := safeRelativePath(targetPath)
	if err != nil {
		return planning.Plan{}, err
	}
	if normalizeCatalogPath(targetRel) == normalizeCatalogPath(currentRel) {
		return planning.Plan{}, fmt.Errorf("preferred placement resolves to the current path")
	}
	if _, err := os.Stat(filepath.Join(s.options.ServerRoot, targetRel)); err == nil {
		return planning.Plan{}, fmt.Errorf("preferred placement target is already occupied: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return planning.Plan{}, err
	}

	key := catalog.EntryKey(entry)
	versionID := managedInstalledVersion(entry)
	release := updatecheck.Release{
		ID: versionID,
		Number: versionID,
		Name: entry.Name,
		Filename: mod.Filename,
		URL: entry.URL,
		SHA1: entry.SHA1,
		SHA512: actual,
	}
	now := time.Now().UTC()
	plan := planning.Plan{
		SchemaVersion: planning.SchemaVersion,
		CreatedAt: now,
		Status: planning.StatusReady,
		InventoryGeneratedAt: s.snapshot.Inventory.GeneratedAt,
		UpdatesGeneratedAt: now,
		Selected: []string{key},
		Verified: true,
		VerifiedAt: &now,
		RequiresServerStop: true,
		RequiresBackup: true,
		Changes: []planning.Change{{
			CandidateKey: key,
			Name: entry.Name,
			Requested: true,
			Classification: updatecheck.ClassificationSafe,
			Installed: release,
			Target: release,
			Artifact: planning.Artifact{
				Provider: entry.Provider,
				ProjectID: entry.ProjectID,
				VersionID: versionID,
				Filename: mod.Filename,
				URL: entry.URL,
				SHA1: entry.SHA1,
				SHA512: actual,
				Deployment: string(entry.Deployment),
			},
			Operations: []planning.FileOperation{{
				Action: "replace",
				CurrentPath: filepath.ToSlash(currentRel),
				TargetPath: filepath.ToSlash(targetRel),
				CurrentSHA512: actual,
				TargetSHA512: actual,
			}},
		}},
	}
	plan.ID = placementPlanID(key, currentRel, targetRel, actual)

	cacheRel := filepath.ToSlash(filepath.Join("cache", "placement", plan.ID, mod.Filename))
	cachePath := filepath.Join(s.options.StateDir, filepath.FromSlash(cacheRel))
	bytes, cachedSHA, err := copyFileWithSHA512(currentPath, cachePath)
	if err != nil {
		return planning.Plan{}, fmt.Errorf("cache current artifact for placement move: %w", err)
	}
	if !strings.EqualFold(cachedSHA, actual) {
		return planning.Plan{}, fmt.Errorf("placement cache verification failed")
	}
	plan.Prefetched = []planning.PrefetchedArtifact{{
		Filename: mod.Filename,
		SHA512: actual,
		CachePath: cacheRel,
		Bytes: bytes,
	}}

	if err := s.createRestorePoint(&plan); err != nil {
		return planning.Plan{}, fmt.Errorf("create placement restore point: %w", err)
	}
	if plan.BackupID == "" {
		return planning.Plan{}, fmt.Errorf("placement plan did not create a restore point")
	}
	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "plans", plan.ID+".json"), plan); err != nil {
		return planning.Plan{}, fmt.Errorf("persist placement plan: %w", err)
	}
	event := planning.HistoryEvent{
		ID: "plan:" + plan.ID,
		CreatedAt: plan.CreatedAt,
		Type: "plan",
		Status: string(plan.Status),
		PlanID: plan.ID,
		BackupID: plan.BackupID,
		Mods: 1,
		Summary: fmt.Sprintf(
			"Placement plan for %s: %s → %s",
			entry.Name,
			mod.Location,
			entry.Deployment,
		),
	}
	if err := s.persistHistoryEvent(event); err != nil {
		return planning.Plan{}, fmt.Errorf("persist placement plan history: %w", err)
	}
	select {
	case <-ctx.Done():
		return planning.Plan{}, ctx.Err()
	default:
	}
	return plan, nil
}

func managedInstalledVersion(entry catalog.Entry) string {
	switch entry.Provider {
	case "github":
		if strings.TrimSpace(entry.Tag) != "" {
			return entry.Tag
		}
	case "curseforge":
		if entry.FileID != 0 {
			return strconv.FormatUint(uint64(entry.FileID), 10)
		}
	}
	return entry.VersionID
}

func placementPlanID(candidateKey, currentPath, targetPath, sha512Value string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		"placement",
		candidateKey,
		filepath.ToSlash(currentPath),
		filepath.ToSlash(targetPath),
		strings.ToLower(sha512Value),
	}, "\x00")))
	return "plan-" + hex.EncodeToString(sum[:8])
}
