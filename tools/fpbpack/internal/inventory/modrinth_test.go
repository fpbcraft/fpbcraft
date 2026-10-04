package inventory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModrinthExactHashMatch(t *testing.T) {
	const hash = "abc123"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/version_files" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		var body struct {
			Hashes    []string `json:"hashes"`
			Algorithm string   `json:"algorithm"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Algorithm != "sha512" || len(body.Hashes) != 1 || body.Hashes[0] != hash {
			t.Fatalf("unexpected request body: %+v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
  "abc123": {
    "id": "version1",
    "project_id": "project1",
    "name": "Example 1.2.3",
    "version_number": "1.2.3",
    "loaders": ["neoforge"],
    "game_versions": ["1.21.1"],
    "environment": "client_and_server",
    "files": [{"hashes":{"sha512":"abc123"},"url":"https://cdn.example/mod.jar","filename":"mod.jar"}]
  }
}`))
	}))
	defer server.Close()

	client := ModrinthClient{BaseURL: server.URL, HTTPClient: server.Client()}
	matches, err := client.Match(context.Background(), []ModFile{{SHA512: hash}})
	if err != nil {
		t.Fatal(err)
	}
	match, ok := matches[hash]
	if !ok {
		t.Fatal("expected exact hash match")
	}
	if match.ProjectID != "project1" || match.VersionID != "version1" || match.Filename != "mod.jar" {
		t.Fatalf("unexpected match: %+v", match)
	}
}
