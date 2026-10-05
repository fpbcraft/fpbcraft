package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModrinthVersionRequestsKeepBroadHistoryLightweight(t *testing.T) {
	var historyQuery map[string]string
	var changelogQuery map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/project/project/version":
			values := r.URL.Query()
			if values.Get("include_changelog") == "true" {
				changelogQuery = map[string]string{
					"include_changelog": values.Get("include_changelog"),
					"game_versions": values.Get("game_versions"),
					"loaders": values.Get("loaders"),
				}
			} else {
				historyQuery = map[string]string{
					"include_changelog": values.Get("include_changelog"),
					"game_versions": values.Get("game_versions"),
					"loaders": values.Get("loaders"),
				}
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &ModrinthClient{
		BaseURL: server.URL,
		HTTPClient: server.Client(),
		Mode: RefreshModeInteractive,
	}
	if _, err := client.ListVersions(context.Background(), "project"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListCompatibleVersionsWithChangelog(
		context.Background(),
		"project",
		"1.21.1",
		"neoforge",
	); err != nil {
		t.Fatal(err)
	}

	if historyQuery["include_changelog"] != "false" {
		t.Fatalf("broad history must omit changelogs: %+v", historyQuery)
	}
	if historyQuery["game_versions"] != "" || historyQuery["loaders"] != "" {
		t.Fatalf("broad history should preserve incompatible versions for rejection reporting: %+v", historyQuery)
	}
	if changelogQuery["include_changelog"] != "true" ||
		changelogQuery["game_versions"] != "[\"1.21.1\"]" ||
		changelogQuery["loaders"] != "[\"neoforge\"]" {
		t.Fatalf("compatible changelog request not filtered as expected: %+v", changelogQuery)
	}
}
