package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
)

func (s *Service) createRestorePoint(plan *planning.Plan) error {
	if plan.Status != planning.StatusReady || !plan.Verified || !plan.RequiresBackup {
		return nil
	}
	backupID := "backup-" + strings.TrimPrefix(plan.ID, "plan-")
	backupDir := filepath.Join(s.options.StateDir, "backups", backupID)
	manifestPath := filepath.Join(backupDir, "manifest.json")

	var existing planning.BackupManifest
	if err := readJSON(manifestPath, &existing); err == nil {
		if existing.PlanID != plan.ID {
			return fmt.Errorf("backup %s belongs to a different plan", backupID)
		}
		plan.BackupID = backupID
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	manifest := planning.BackupManifest{
		SchemaVersion: 1,
		ID: backupID,
		PlanID: plan.ID,
		CreatedAt: time.Now().UTC(),
	}
	for _, change := range plan.Changes {
		for _, operation := range change.Operations {
			if operation.Action != "replace" || operation.CurrentPath == "" {
				continue
			}
			relative, err := safeRelativePath(operation.CurrentPath)
			if err != nil {
				return fmt.Errorf("%s: %w", change.Name, err)
			}
			source := filepath.Join(s.options.ServerRoot, relative)
			if operation.CurrentSHA512 != "" {
				actual, err := sha512File(source)
				if err != nil {
					return fmt.Errorf("hash current artifact %s: %w", operation.CurrentPath, err)
				}
				if !strings.EqualFold(actual, operation.CurrentSHA512) {
					return fmt.Errorf("current artifact changed before backup: %s", operation.CurrentPath)
				}
			}

			backupRelative := filepath.Join("files", relative)
			target := filepath.Join(backupDir, backupRelative)
			bytes, sha, err := copyFileWithSHA512(source, target)
			if err != nil {
				return fmt.Errorf("backup %s: %w", operation.CurrentPath, err)
			}
			if operation.CurrentSHA512 != "" && !strings.EqualFold(sha, operation.CurrentSHA512) {
				return fmt.Errorf("backup hash mismatch for %s", operation.CurrentPath)
			}
			manifest.Files = append(manifest.Files, planning.BackupFile{
				SourcePath: filepath.ToSlash(relative),
				BackupPath: filepath.ToSlash(backupRelative),
				SHA512: sha,
				Bytes: bytes,
			})
		}
	}
	if len(manifest.Files) == 0 {
		return fmt.Errorf("plan requires a restore point but has no replace operations to back up")
	}
	if err := writeJSONAtomic(manifestPath, manifest); err != nil {
		return err
	}
	plan.BackupID = backupID
	return nil
}

func safeRelativePath(value string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe managed path %q", value)
	}
	return clean, nil
}

func copyFileWithSHA512(source, target string) (int64, string, error) {
	input, err := os.Open(source)
	if err != nil {
		return 0, "", err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return 0, "", err
	}
	if !info.Mode().IsRegular() {
		return 0, "", fmt.Errorf("source is not a regular file")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".fpbpack-backup-*")
	if err != nil {
		return 0, "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	hashWriter := newSHA512Writer()
	written, err := io.Copy(io.MultiWriter(tmp, hashWriter), input)
	if err != nil {
		_ = tmp.Close()
		return 0, "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return 0, "", err
	}
	if err := tmp.Close(); err != nil {
		return 0, "", err
	}
	if err := os.Chmod(tmpPath, info.Mode().Perm()); err != nil {
		return 0, "", err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return 0, "", err
	}
	return written, hashWriter.Sum(), nil
}
