package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetProviderCredentialValidatesAndStoresSecret0600(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/games/432" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("x-api-key") != "valid-key" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"id": 432},
		})
	}))
	defer server.Close()

	stateDir := t.TempDir()
	s := &Service{
		options: Options{
			StateDir: stateDir,
			CurseForgeBaseURL: server.URL,
		},
		secrets: ProviderSecrets{SchemaVersion: ProviderSecretsSchemaVersion},
	}

	status, err := s.SetProviderCredential(context.Background(), "curseforge", "valid-key")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "ready" || status.CredentialSource != "saved" {
		t.Fatalf("unexpected provider status: %+v", status)
	}

	path := filepath.Join(stateDir, "secrets.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("secrets.json mode = %o, want 600", got)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bytes), "valid-key") {
		t.Fatal("saved credential is missing from secrets.json")
	}

	key, source := s.effectiveCurseForgeAPIKey()
	if key != "valid-key" || source != "saved" {
		t.Fatalf("effective credential = %q/%q", key, source)
	}
}

func TestSetProviderCredentialRejectsInvalidKeyWithoutReplacingExisting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	s := &Service{
		options: Options{
			StateDir: t.TempDir(),
			CurseForgeBaseURL: server.URL,
		},
		secrets: ProviderSecrets{
			SchemaVersion: ProviderSecretsSchemaVersion,
			CurseForgeAPIKey: "old-key",
		},
	}
	if _, err := s.SetProviderCredential(context.Background(), "curseforge", "bad-key"); err == nil {
		t.Fatal("expected invalid key to fail")
	}
	key, source := s.effectiveCurseForgeAPIKey()
	if key != "old-key" || source != "saved" {
		t.Fatalf("existing credential was replaced after validation failure: %q/%q", key, source)
	}
}

func TestClearProviderCredentialFallsBackToEnvironment(t *testing.T) {
	stateDir := t.TempDir()
	s := &Service{
		options: Options{
			StateDir: stateDir,
			CurseForgeAPIKey: "environment-key",
		},
		secrets: ProviderSecrets{
			SchemaVersion: ProviderSecretsSchemaVersion,
			CurseForgeAPIKey: "saved-key",
		},
	}
	status, err := s.ClearProviderCredential("curseforge")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "ready" || status.CredentialSource != "environment" {
		t.Fatalf("unexpected provider status after clear: %+v", status)
	}
	key, source := s.effectiveCurseForgeAPIKey()
	if key != "environment-key" || source != "environment" {
		t.Fatalf("fallback credential = %q/%q", key, source)
	}
}
