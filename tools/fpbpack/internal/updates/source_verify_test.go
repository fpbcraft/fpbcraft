package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyInstalledModrinthVersionRequiresExactArtifactHash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/version/v1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "v1",
			"project_id": "project",
			"name": "Version 1",
			"version_number": "1.0.0",
			"environment": "client_and_server",
			"files": []map[string]any{{
				"filename": "example.jar",
				"url": "https://cdn.example/example.jar",
				"primary": true,
				"hashes": map[string]any{"sha512": "abc123"},
			}},
		})
	}))
	defer server.Close()

	client := ModrinthClient{
		BaseURL: server.URL,
		HTTPClient: server.Client(),
		Mode: RefreshModeInteractive,
	}
	verified, err := client.VerifyInstalledVersion(
		context.Background(),
		"project",
		"v1",
		"example.jar",
		"abc123",
	)
	if err != nil {
		t.Fatal(err)
	}
	if verified.ProjectID != "project" || verified.VersionID != "v1" || verified.Filename != "example.jar" {
		t.Fatalf("unexpected verification result: %+v", verified)
	}

	if _, err := client.VerifyInstalledVersion(
		context.Background(),
		"project",
		"v1",
		"example.jar",
		"different",
	); err == nil {
		t.Fatal("expected mismatched installed SHA-512 to be rejected")
	}
}

func TestVerifyInstalledCurseForgeFileRequiresExactArtifactHash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/mods/123/files/10" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id": 10,
				"modId": 123,
				"displayName": "Version 1",
				"fileName": "example.jar",
				"isAvailable": true,
				"hashes": []map[string]any{{
					"value": "sha1-value",
					"algo": 1,
				}},
			},
		})
	}))
	defer server.Close()

	client := CurseForgeClient{
		BaseURL: server.URL,
		APIKey: "test-key",
		HTTPClient: server.Client(),
		Mode: RefreshModeInteractive,
	}
	verified, err := client.VerifyInstalledFile(
		context.Background(),
		"123",
		10,
		"sha1-value",
	)
	if err != nil {
		t.Fatal(err)
	}
	if verified.ProjectID != "123" || verified.FileID != 10 || verified.Filename != "example.jar" {
		t.Fatalf("unexpected verification result: %+v", verified)
	}

	if _, err := client.VerifyInstalledFile(
		context.Background(),
		"123",
		10,
		"different",
	); err == nil {
		t.Fatal("expected mismatched installed SHA-1 to be rejected")
	}
}


func TestMatchCurseForgeFingerprintsReturnsExactProviderIdentity(t *testing.T) {
	const fingerprint uint32 = 197930586
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/fingerprints/432" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing CurseForge API key")
		}
		var body struct {
			Fingerprints []uint32 `json:"fingerprints"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Fingerprints) != 1 || body.Fingerprints[0] != fingerprint {
			t.Fatalf("fingerprints = %+v", body.Fingerprints)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"exactMatches": []map[string]any{{
					"id": fingerprint,
					"file": map[string]any{
						"id": 999,
						"modId": 123,
						"isAvailable": true,
						"displayName": "Example 2.0",
						"fileName": "example.jar",
						"releaseType": 1,
						"fileFingerprint": fingerprint,
						"gameVersions": []string{"1.21.1", "NeoForge"},
					},
				}},
			},
		})
	}))
	defer server.Close()

	client := CurseForgeClient{
		BaseURL: server.URL,
		APIKey: "test-key",
		HTTPClient: server.Client(),
		Mode: RefreshModeInteractive,
	}
	matches, err := client.MatchFingerprints(context.Background(), []uint32{fingerprint, fingerprint, 0})
	if err != nil {
		t.Fatal(err)
	}
	match, ok := matches[fingerprint]
	if !ok {
		t.Fatalf("no match returned: %+v", matches)
	}
	if match.ProjectID != 123 || match.FileID != 999 || match.Filename != "example.jar" {
		t.Fatalf("unexpected match: %+v", match)
	}
}
