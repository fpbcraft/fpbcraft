package service

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
)

const maxArtifactBytes int64 = 2 << 30

type verifiedArtifact struct {
	Bytes  int64
	SHA1   string
	SHA256 string
	SHA512 string
}

func (s *Service) verifyPlanArtifacts(ctx context.Context, plan *planning.Plan) {
	if plan.Status == planning.StatusBlocked {
		return
	}

	for index := range plan.Changes {
		change := &plan.Changes[index]
		needsTargetArtifact := false
		for _, operation := range change.Operations {
			if operation.Action != "remove" {
				needsTargetArtifact = true
				break
			}
		}
		if !needsTargetArtifact {
			continue
		}
		cacheKey := change.Artifact.SHA512
		if cacheKey == "" && change.Artifact.SHA256 != "" {
			cacheKey = "sha256-" + change.Artifact.SHA256
		}
		if cacheKey == "" {
			cacheKey = "sha1-" + change.Artifact.SHA1
		}
		cacheRelative := filepath.ToSlash(filepath.Join("cache", "artifacts", cacheKey+".jar"))
		cachePath := filepath.Join(s.options.StateDir, filepath.FromSlash(cacheRelative))

		if sourcePath, ok := reusableCurrentArtifact(*change); ok {
			relative, relErr := safeRelativePath(sourcePath)
			if relErr != nil {
				plan.Blockers = append(plan.Blockers, planning.Finding{
					Code: "artifact_verification_failed", CandidateKey: change.CandidateKey,
					Name: change.Name, Path: sourcePath, Message: relErr.Error(),
				})
				continue
			}
			source := filepath.Join(s.options.ServerRoot, relative)
			hashes, hashErr := artifactFileHashes(source)
			if hashErr != nil || !hashesMatch(
				hashes,
				change.Artifact.SHA512,
				change.Artifact.SHA256,
				change.Artifact.SHA1,
			) {
				message := "current artifact cannot be reused for the staged placement move"
				if hashErr != nil {
					message += ": " + hashErr.Error()
				}
				plan.Blockers = append(plan.Blockers, planning.Finding{
					Code: "artifact_verification_failed", CandidateKey: change.CandidateKey,
					Name: change.Name, Path: sourcePath, Message: message,
				})
				continue
			}
			bytes, copiedSHA, copyErr := copyFileWithSHA512(source, cachePath)
			if copyErr != nil || !strings.EqualFold(copiedSHA, hashes.SHA512) {
				message := "cache current artifact for placement move"
				if copyErr != nil {
					message += ": " + copyErr.Error()
				}
				plan.Blockers = append(plan.Blockers, planning.Finding{
					Code: "artifact_verification_failed", CandidateKey: change.CandidateKey,
					Name: change.Name, Path: sourcePath, Message: message,
				})
				continue
			}
			hashes.Bytes = bytes
			verified := hashes
			change.Artifact.SHA512 = verified.SHA512
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
			plan.Prefetched = append(plan.Prefetched, planning.PrefetchedArtifact{
				Filename: change.Artifact.Filename,
				SHA512: verified.SHA512,
				CachePath: cacheRelative,
				Bytes: verified.Bytes,
			})
			continue
		}

		if change.Artifact.ManualDownload {
			message := "This artifact must be downloaded manually from the provider before Apply."
			if change.Artifact.ManualURL != "" {
				message += " Manual download: " + change.Artifact.ManualURL
			}
			path := change.Artifact.Filename
			if len(change.Operations) > 0 {
				path = change.Operations[0].TargetPath
			}
			plan.Blockers = append(plan.Blockers, planning.Finding{
				Code:         "manual_download_required",
				CandidateKey: change.CandidateKey,
				Name:         change.Name,
				Path:         path,
				Message:      message,
			})
			continue
		}
		verified, err := ensureArtifact(
			ctx,
			change.Artifact.URL,
			change.Artifact.SHA512,
			change.Artifact.SHA256,
			change.Artifact.SHA1,
			cachePath,
		)
		if err != nil {
			plan.Blockers = append(plan.Blockers, planning.Finding{
				Code: "artifact_verification_failed",
				CandidateKey: change.CandidateKey,
				Message: fmt.Sprintf("%s: %v", change.Name, err),
			})
			continue
		}

		change.Artifact.SHA512 = verified.SHA512
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

		plan.Prefetched = append(plan.Prefetched, planning.PrefetchedArtifact{
			Filename: change.Artifact.Filename,
			SHA512: verified.SHA512,
			CachePath: cacheRelative,
			Bytes: verified.Bytes,
		})
	}

	if len(plan.Blockers) > 0 {
		plan.Status = planning.StatusBlocked
		plan.Verified = false
		plan.VerifiedAt = nil
		return
	}
	now := time.Now().UTC()
	plan.Verified = true
	plan.VerifiedAt = &now
}

func reusableCurrentArtifact(change planning.Change) (string, bool) {
	if len(change.Operations) != 1 {
		return "", false
	}
	operation := change.Operations[0]
	if operation.Action != "replace" ||
		strings.TrimSpace(operation.CurrentPath) == "" ||
		strings.TrimSpace(operation.TargetPath) == "" ||
		filepath.ToSlash(filepath.Clean(operation.CurrentPath)) ==
			filepath.ToSlash(filepath.Clean(operation.TargetPath)) {
		return "", false
	}
	targetHash := strings.TrimSpace(change.Artifact.SHA512)
	if targetHash == "" {
		targetHash = strings.TrimSpace(change.Target.SHA512)
	}
	return operation.CurrentPath,
		targetHash != "" &&
			strings.EqualFold(strings.TrimSpace(operation.CurrentSHA512), targetHash)
}

func ensureArtifact(
	ctx context.Context,
	artifactURL string,
	expectedSHA512 string,
	expectedSHA256 string,
	expectedSHA1 string,
	targetPath string,
) (verifiedArtifact, error) {
	if strings.TrimSpace(artifactURL) == "" ||
		(strings.TrimSpace(expectedSHA512) == "" && strings.TrimSpace(expectedSHA256) == "" && strings.TrimSpace(expectedSHA1) == "") {
		return verifiedArtifact{}, fmt.Errorf("artifact URL and at least one provider checksum are required")
	}
	parsed, err := url.Parse(artifactURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return verifiedArtifact{}, fmt.Errorf("unsupported artifact URL")
	}

	if info, err := os.Stat(targetPath); err == nil && info.Mode().IsRegular() {
		hashes, hashErr := artifactFileHashes(targetPath)
		if hashErr == nil && hashesMatch(hashes, expectedSHA512, expectedSHA256, expectedSHA1) {
			hashes.Bytes = info.Size()
			return hashes, nil
		}
		if removeErr := os.Remove(targetPath); removeErr != nil {
			return verifiedArtifact{}, fmt.Errorf("remove invalid cached artifact: %w", removeErr)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return verifiedArtifact{}, err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return verifiedArtifact{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifactURL, nil)
	if err != nil {
		return verifiedArtifact{}, err
	}
	request.Header.Set("User-Agent", "fpbpack/plan-prefetch")
	client := &http.Client{Timeout: 10 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return verifiedArtifact{}, fmt.Errorf("download target artifact: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return verifiedArtifact{}, fmt.Errorf("download target artifact returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxArtifactBytes {
		return verifiedArtifact{}, fmt.Errorf("artifact is larger than the %d byte safety limit", maxArtifactBytes)
	}

	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".fpbpack-artifact-*")
	if err != nil {
		return verifiedArtifact{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	sha1Hash := sha1.New()
	sha256Hash := sha256.New()
	sha512Hash := sha512.New()
	written, copyErr := io.Copy(
		io.MultiWriter(tmp, sha1Hash, sha256Hash, sha512Hash),
		io.LimitReader(response.Body, maxArtifactBytes+1),
	)
	if copyErr != nil {
		_ = tmp.Close()
		return verifiedArtifact{}, copyErr
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
			return verifiedArtifact{}, fmt.Errorf("SHA-512 mismatch: expected %s, got %s", expectedSHA512, hashes.SHA512)
		}
		if expectedSHA256 != "" && !strings.EqualFold(hashes.SHA256, expectedSHA256) {
			return verifiedArtifact{}, fmt.Errorf("SHA-256 mismatch: expected %s, got %s", expectedSHA256, hashes.SHA256)
		}
		return verifiedArtifact{}, fmt.Errorf("SHA-1 mismatch: expected %s, got %s", expectedSHA1, hashes.SHA1)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return verifiedArtifact{}, err
	}
	if err := tmp.Close(); err != nil {
		return verifiedArtifact{}, err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return verifiedArtifact{}, err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return verifiedArtifact{}, err
	}
	return hashes, nil
}

func hashesMatch(actual verifiedArtifact, expectedSHA512, expectedSHA256, expectedSHA1 string) bool {
	if expectedSHA512 != "" && !strings.EqualFold(actual.SHA512, expectedSHA512) {
		return false
	}
	if expectedSHA256 != "" && !strings.EqualFold(actual.SHA256, expectedSHA256) {
		return false
	}
	if expectedSHA1 != "" && !strings.EqualFold(actual.SHA1, expectedSHA1) {
		return false
	}
	return true
}

func artifactFileHashes(path string) (verifiedArtifact, error) {
	file, err := os.Open(path)
	if err != nil {
		return verifiedArtifact{}, err
	}
	defer file.Close()
	sha1Hash := sha1.New()
	sha256Hash := sha256.New()
	sha512Hash := sha512.New()
	if _, err := io.Copy(io.MultiWriter(sha1Hash, sha256Hash, sha512Hash), file); err != nil {
		return verifiedArtifact{}, err
	}
	return verifiedArtifact{
		SHA1: hex.EncodeToString(sha1Hash.Sum(nil)),
		SHA256: hex.EncodeToString(sha256Hash.Sum(nil)),
		SHA512: hex.EncodeToString(sha512Hash.Sum(nil)),
	}, nil
}

func sha512File(path string) (string, error) {
	hashes, err := artifactFileHashes(path)
	if err != nil {
		return "", err
	}
	return hashes.SHA512, nil
}
