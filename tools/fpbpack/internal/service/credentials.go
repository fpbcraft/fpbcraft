package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

const ProviderSecretsSchemaVersion = 1

type ProviderSecrets struct {
	SchemaVersion      int    `json:"schema_version"`
	CurseForgeAPIKey   string `json:"curseforge_api_key,omitempty"`
}

func (s *Service) loadProviderSecrets() error {
	path := filepath.Join(s.options.StateDir, "secrets.json")
	var secrets ProviderSecrets
	if err := readJSON(path, &secrets); err != nil {
		if os.IsNotExist(err) {
			s.secrets = ProviderSecrets{SchemaVersion: ProviderSecretsSchemaVersion}
			return nil
		}
		return fmt.Errorf("load provider secrets: %w", err)
	}
	if secrets.SchemaVersion != ProviderSecretsSchemaVersion {
		return fmt.Errorf(
			"unsupported provider secrets schema %d (expected %d)",
			secrets.SchemaVersion,
			ProviderSecretsSchemaVersion,
		)
	}
	s.secrets = secrets
	return nil
}

func (s *Service) effectiveCurseForgeAPIKey() (string, string) {
	s.mu.RLock()
	saved := strings.TrimSpace(s.secrets.CurseForgeAPIKey)
	s.mu.RUnlock()
	if saved != "" {
		return saved, "saved"
	}
	if env := strings.TrimSpace(s.options.CurseForgeAPIKey); env != "" {
		return env, "environment"
	}
	return "", ""
}

func (s *Service) SetProviderCredential(
	ctx context.Context,
	provider string,
	apiKey string,
) (ProviderStatus, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	apiKey = strings.TrimSpace(apiKey)
	if provider != "curseforge" {
		return ProviderStatus{}, fmt.Errorf("provider %q does not support GUI credentials", provider)
	}
	if apiKey == "" {
		return ProviderStatus{}, fmt.Errorf("CurseForge API key is required")
	}

	validateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := updatecheck.CurseForgeClient{
		BaseURL: s.options.CurseForgeBaseURL,
		APIKey:  apiKey,
	}
	if err := client.Validate(validateCtx); err != nil {
		return ProviderStatus{}, fmt.Errorf("validate CurseForge API key: %w", err)
	}

	s.mu.RLock()
	next := s.secrets
	s.mu.RUnlock()
	next.SchemaVersion = ProviderSecretsSchemaVersion
	next.CurseForgeAPIKey = apiKey

	if err := writeSecretJSONAtomic(filepath.Join(s.options.StateDir, "secrets.json"), next); err != nil {
		return ProviderStatus{}, fmt.Errorf("persist provider credential: %w", err)
	}
	s.mu.Lock()
	s.secrets = next
	s.mu.Unlock()
	return s.providerStatus("curseforge"), nil
}

func (s *Service) ClearProviderCredential(provider string) (ProviderStatus, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "curseforge" {
		return ProviderStatus{}, fmt.Errorf("provider %q does not support GUI credentials", provider)
	}

	s.mu.RLock()
	next := s.secrets
	s.mu.RUnlock()
	next.SchemaVersion = ProviderSecretsSchemaVersion
	next.CurseForgeAPIKey = ""

	if err := writeSecretJSONAtomic(filepath.Join(s.options.StateDir, "secrets.json"), next); err != nil {
		return ProviderStatus{}, fmt.Errorf("persist provider credential: %w", err)
	}
	s.mu.Lock()
	s.secrets = next
	s.mu.Unlock()
	return s.providerStatus("curseforge"), nil
}

func writeSecretJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".fpbpack-secret-*.json")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)

	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = file.Close()
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
