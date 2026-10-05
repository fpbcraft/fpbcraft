package service

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
)

func (s *Service) AcceptManualArtifact(
	ctx context.Context,
	planID string,
	candidateKey string,
	reader io.Reader,
) (planning.Plan, error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	candidateKey = strings.TrimSpace(candidateKey)
	if candidateKey == "" {
		return planning.Plan{}, fmt.Errorf("candidate key is required")
	}
	if reader == nil {
		return planning.Plan{}, fmt.Errorf("manual artifact body is required")
	}

	plan, err := s.Plan(planID)
	if err != nil {
		return planning.Plan{}, err
	}
	if plan.AppliedAt != nil {
		return planning.Plan{}, fmt.Errorf("plan %s has already been applied", plan.ID)
	}

	changeIndex := -1
	for index := range plan.Changes {
		if plan.Changes[index].CandidateKey == candidateKey {
			changeIndex = index
			break
		}
	}
	if changeIndex < 0 {
		return planning.Plan{}, fmt.Errorf("candidate %q is not part of plan %s", candidateKey, plan.ID)
	}
	change := &plan.Changes[changeIndex]
	if !change.Artifact.ManualDownload {
		return planning.Plan{}, fmt.Errorf("%s does not require a manual provider artifact", change.Name)
	}
	if strings.TrimSpace(change.Artifact.SHA512) == "" &&
		strings.TrimSpace(change.Artifact.SHA256) == "" &&
		strings.TrimSpace(change.Artifact.SHA1) == "" {
		return planning.Plan{}, fmt.Errorf("%s has no provider checksum for manual verification", change.Name)
	}

	cacheRelative := filepath.ToSlash(filepath.Join(
		"cache",
		"artifacts",
		"manual",
		plan.ID,
		fmt.Sprintf("%02d-%s", changeIndex, filepath.Base(change.Artifact.Filename)),
	))
	cachePath := filepath.Join(s.options.StateDir, filepath.FromSlash(cacheRelative))
	verified, err := storeManualArtifact(
		ctx,
		reader,
		change.Artifact.SHA512,
		change.Artifact.SHA256,
		change.Artifact.SHA1,
		cachePath,
	)
	if err != nil {
		return planning.Plan{}, fmt.Errorf("verify manual artifact for %s: %w", change.Name, err)
	}

	change.Artifact.SHA512 = verified.SHA512
	change.Artifact.ManualProvided = true
	change.Target.SHA512 = verified.SHA512
	if change.Artifact.SHA1 == "" {
		change.Artifact.SHA1 = verified.SHA1
	}
	if change.Artifact.SHA256 == "" {
		change.Artifact.SHA256 = verified.SHA256
	}
	if change.Target.SHA1 == "" {
		change.Target.SHA1 = verified.SHA1
	}
	if change.Target.SHA256 == "" {
		change.Target.SHA256 = verified.SHA256
	}
	for operationIndex := range change.Operations {
		change.Operations[operationIndex].TargetSHA512 = verified.SHA512
	}

	replacedPrefetch := false
	for index := range plan.Prefetched {
		if plan.Prefetched[index].CachePath == cacheRelative {
			plan.Prefetched[index] = planning.PrefetchedArtifact{
				Filename: change.Artifact.Filename,
				SHA512: verified.SHA512,
				CachePath: cacheRelative,
				Bytes: verified.Bytes,
			}
			replacedPrefetch = true
			break
		}
	}
	if !replacedPrefetch {
		plan.Prefetched = append(plan.Prefetched, planning.PrefetchedArtifact{
			Filename: change.Artifact.Filename,
			SHA512: verified.SHA512,
			CachePath: cacheRelative,
			Bytes: verified.Bytes,
		})
	}

	filtered := plan.Blockers[:0]
	for _, blocker := range plan.Blockers {
		if blocker.Code == "manual_download_required" && blocker.CandidateKey == candidateKey {
			continue
		}
		filtered = append(filtered, blocker)
	}
	plan.Blockers = filtered

	allManualReady := true
	for _, plannedChange := range plan.Changes {
		if plannedChange.Artifact.ManualDownload && !plannedChange.Artifact.ManualProvided {
			allManualReady = false
			break
		}
	}
	if len(plan.Blockers) == 0 && allManualReady {
		now := time.Now().UTC()
		plan.Status = planning.StatusReady
		plan.Verified = true
		plan.VerifiedAt = &now
		if err := s.createRestorePoint(&plan); err != nil {
			plan.Status = planning.StatusBlocked
			plan.Verified = false
			plan.VerifiedAt = nil
			plan.Blockers = append(plan.Blockers, planning.Finding{
				Code: "backup_creation_failed",
				Message: err.Error(),
			})
		}
	}

	if err := writeJSONAtomic(filepath.Join(s.options.StateDir, "plans", plan.ID+".json"), plan); err != nil {
		return planning.Plan{}, fmt.Errorf("persist manual artifact verification: %w", err)
	}
	now := time.Now().UTC()
	event := planning.HistoryEvent{
		ID: "manual-artifact:" + plan.ID + ":" + fmt.Sprintf("%02d", changeIndex) + ":" + now.Format("20060102T150405.000000000Z"),
		CreatedAt: now,
		Type: "manual_artifact",
		Status: "verified",
		PlanID: plan.ID,
		BackupID: plan.BackupID,
		Mods: 1,
		Summary: "Verified manually downloaded artifact for " + change.Name,
	}
	if err := s.persistHistoryEvent(event); err != nil {
		return planning.Plan{}, fmt.Errorf("persist manual artifact history: %w", err)
	}
	return plan, nil
}

func storeManualArtifact(
	ctx context.Context,
	reader io.Reader,
	expectedSHA512 string,
	expectedSHA256 string,
	expectedSHA1 string,
	targetPath string,
) (verifiedArtifact, error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return verifiedArtifact{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".fpbpack-manual-*")
	if err != nil {
		return verifiedArtifact{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	sha1Hash := sha1.New()
	sha256Hash := sha256.New()
	sha512Hash := sha512.New()
	limited := io.LimitReader(reader, maxArtifactBytes+1)
	written, err := io.Copy(io.MultiWriter(tmp, sha1Hash, sha256Hash, sha512Hash), &contextReader{
		ctx: ctx,
		reader: limited,
	})
	if err != nil {
		_ = tmp.Close()
		return verifiedArtifact{}, err
	}
	if written > maxArtifactBytes {
		_ = tmp.Close()
		return verifiedArtifact{}, fmt.Errorf("artifact exceeded the %d byte safety limit", maxArtifactBytes)
	}
	hashes := verifiedArtifact{
		Bytes: written,
		SHA1: hex.EncodeToString(sha1Hash.Sum(nil)),
		SHA256: hex.EncodeToString(sha256Hash.Sum(nil)),
		SHA512: hex.EncodeToString(sha512Hash.Sum(nil)),
	}
	if !hashesMatch(hashes, expectedSHA512, expectedSHA256, expectedSHA1) {
		_ = tmp.Close()
		if expectedSHA512 != "" && !strings.EqualFold(hashes.SHA512, expectedSHA512) {
			return verifiedArtifact{}, fmt.Errorf("SHA-512 mismatch")
		}
		if expectedSHA256 != "" && !strings.EqualFold(hashes.SHA256, expectedSHA256) {
			return verifiedArtifact{}, fmt.Errorf("SHA-256 mismatch")
		}
		return verifiedArtifact{}, fmt.Errorf("SHA-1 mismatch")
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return verifiedArtifact{}, err
	}
	if err := tmp.Close(); err != nil {
		return verifiedArtifact{}, err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return verifiedArtifact{}, err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return verifiedArtifact{}, err
	}
	return hashes, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(buffer)
	}
}
