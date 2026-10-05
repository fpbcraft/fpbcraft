package service

import (
	"context"
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

func (s *Service) verifyPlanArtifacts(ctx context.Context, plan *planning.Plan) {
	if plan.Status == planning.StatusBlocked {
		return
	}

	for _, change := range plan.Changes {
		cacheRelative := filepath.ToSlash(filepath.Join("cache", "artifacts", change.Artifact.SHA512+".jar"))
		cachePath := filepath.Join(s.options.StateDir, filepath.FromSlash(cacheRelative))
		size, err := ensureArtifact(ctx, change.Artifact.URL, change.Artifact.SHA512, cachePath)
		if err != nil {
			plan.Blockers = append(plan.Blockers, planning.Finding{
				Code: "artifact_verification_failed",
				CandidateKey: change.CandidateKey,
				Message: fmt.Sprintf("%s: %v", change.Name, err),
			})
			continue
		}
		plan.Prefetched = append(plan.Prefetched, planning.PrefetchedArtifact{
			Filename: change.Artifact.Filename,
			SHA512: change.Artifact.SHA512,
			CachePath: cacheRelative,
			Bytes: size,
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

func ensureArtifact(ctx context.Context, artifactURL, expectedSHA512, targetPath string) (int64, error) {
	if strings.TrimSpace(artifactURL) == "" || strings.TrimSpace(expectedSHA512) == "" {
		return 0, fmt.Errorf("artifact URL and SHA-512 are required")
	}
	parsed, err := url.Parse(artifactURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return 0, fmt.Errorf("unsupported artifact URL")
	}

	if info, err := os.Stat(targetPath); err == nil && info.Mode().IsRegular() {
		actual, hashErr := sha512File(targetPath)
		if hashErr == nil && strings.EqualFold(actual, expectedSHA512) {
			return info.Size(), nil
		}
		if removeErr := os.Remove(targetPath); removeErr != nil {
			return 0, fmt.Errorf("remove invalid cached artifact: %w", removeErr)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return 0, err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifactURL, nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("User-Agent", "fpbpack/plan-prefetch")
	client := &http.Client{Timeout: 10 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("download target artifact: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return 0, fmt.Errorf("download target artifact returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxArtifactBytes {
		return 0, fmt.Errorf("artifact is larger than the %d byte safety limit", maxArtifactBytes)
	}

	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".fpbpack-artifact-*")
	if err != nil {
		return 0, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	hash := sha512.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(response.Body, maxArtifactBytes+1))
	if copyErr != nil {
		_ = tmp.Close()
		return 0, copyErr
	}
	if written > maxArtifactBytes {
		_ = tmp.Close()
		return 0, fmt.Errorf("artifact exceeded the %d byte safety limit", maxArtifactBytes)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expectedSHA512) {
		_ = tmp.Close()
		return 0, fmt.Errorf("SHA-512 mismatch: expected %s, got %s", expectedSHA512, actual)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return 0, err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return 0, err
	}
	return written, nil
}

func sha512File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha512.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
