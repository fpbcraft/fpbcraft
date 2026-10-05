package inventory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurseForgeExactFingerprintMatch(t *testing.T) {
	const fingerprint uint32 = 123456789
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/fingerprints" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing API key: %q", request.Header.Get("x-api-key"))
		}
		var body struct {
			Fingerprints []uint32 `json:"fingerprints"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Fingerprints) != 1 || body.Fingerprints[0] != fingerprint {
			t.Fatalf("unexpected request body: %+v", body)
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"data": {
				"exactMatches": [{
					"id": 326652,
					"file": {
						"id": 8889050,
						"modId": 326652,
						"fileFingerprint": 123456789,
						"isAvailable": true,
						"displayName": "Cupboard 1.21.1-4.2",
						"fileName": "cupboard-1.21.1-4.2.jar",
						"releaseType": 1,
						"gameVersions": ["1.21.1", "NeoForge"]
					}
				}]
			}
		}`))
	}))
	defer server.Close()

	client := CurseForgeClient{BaseURL: server.URL, APIKey: "test-key", HTTPClient: server.Client()}
	matches, err := client.Match(context.Background(), []ModFile{{CurseForgeFingerprint: fingerprint}})
	if err != nil {
		t.Fatal(err)
	}
	match, ok := matches[fingerprint]
	if !ok {
		t.Fatal("expected exact fingerprint match")
	}
	if match.ProjectID != 326652 || match.FileID != 8889050 || match.Filename != "cupboard-1.21.1-4.2.jar" {
		t.Fatalf("unexpected match: %+v", match)
	}
}

func TestCurseForgeSkipsModrinthAndUnavailableFiles(t *testing.T) {
	const fingerprint uint32 = 555
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"data": {
				"exactMatches": [{
					"id": 2,
					"file": {
						"id": 1,
						"modId": 2,
						"fileFingerprint": 555,
						"isAvailable": false,
						"fileName": "unavailable.jar"
					}
				}]
			}
		}`))
	}))
	defer server.Close()

	client := CurseForgeClient{BaseURL: server.URL, APIKey: "test-key", HTTPClient: server.Client()}
	matches, err := client.Match(context.Background(), []ModFile{
		{CurseForgeFingerprint: fingerprint},
		{CurseForgeFingerprint: 999, Modrinth: &ModrinthMatch{ProjectID: "already-matched"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no usable matches, got %+v", matches)
	}
}

func TestApplyCurseForgeMatchesPreservesModrinthPriority(t *testing.T) {
	result := Inventory{
		SchemaVersion:     SchemaVersion,
		ModrinthChecked:   true,
		CurseForgeChecked: false,
		Mods: []ModFile{
			{CurseForgeFingerprint: 1, Modrinth: &ModrinthMatch{ProjectID: "mr"}},
			{CurseForgeFingerprint: 2},
		},
	}
	ApplyCurseForgeMatches(&result, map[uint32]CurseForgeMatch{
		1: {ProjectID: 10, FileID: 11},
		2: {ProjectID: 20, FileID: 21},
	})
	if result.Mods[0].CurseForge != nil {
		t.Fatal("CurseForge must not replace an exact Modrinth mapping")
	}
	if result.Mods[1].CurseForge == nil || result.Mods[1].CurseForge.ProjectID != 20 {
		t.Fatalf("expected CurseForge mapping on unmatched mod: %+v", result.Mods[1])
	}
}
