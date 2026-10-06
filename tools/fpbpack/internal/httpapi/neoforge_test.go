package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/service"
)

func TestNeoForgeManagementRoutes(t *testing.T) {
	changedTo := ""
	handler := NewHandlerWithOptions(nil, "dev", ServerOptions{
		NeoForgeStatus: func(context.Context) (service.NeoForgeStatus, error) {
			return service.NeoForgeStatus{
				Minecraft:      "1.21.1",
				CurrentVersion: "21.1.180",
				LatestVersion:  "21.1.201",
				ServerState:    "stopped",
				Versions: []service.NeoForgeVersion{
					{Version: "21.1.201", Channel: "release"},
					{Version: "21.1.180", Channel: "release", Current: true},
				},
			}, nil
		},
		ChangeNeoForge: func(_ context.Context, version string) (service.NeoForgeChangeResult, error) {
			changedTo = version
			return service.NeoForgeChangeResult{
				FromVersion: "21.1.180",
				ToVersion:   version,
				Direction:   "upgrade",
			}, nil
		},
	})

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/neoforge", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET /api/neoforge = %d, want 200: %s", get.Code, get.Body.String())
	}
	var status service.NeoForgeStatus
	if err := json.Unmarshal(get.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.CurrentVersion != "21.1.180" || status.LatestVersion != "21.1.201" {
		t.Fatalf("unexpected NeoForge status: %+v", status)
	}

	post := httptest.NewRecorder()
	handler.ServeHTTP(
		post,
		httptest.NewRequest(
			http.MethodPost,
			"/api/neoforge/change",
			strings.NewReader(`{"version":"21.1.201"}`),
		),
	)
	if post.Code != http.StatusOK {
		t.Fatalf("POST /api/neoforge/change = %d, want 200: %s", post.Code, post.Body.String())
	}
	if changedTo != "21.1.201" {
		t.Fatalf("changed version = %q, want 21.1.201", changedTo)
	}
}

func TestNeoForgeChangeRequiresVersion(t *testing.T) {
	handler := NewHandlerWithOptions(nil, "dev", ServerOptions{
		ChangeNeoForge: func(context.Context, string) (service.NeoForgeChangeResult, error) {
			t.Fatal("changer should not be called for an empty version")
			return service.NeoForgeChangeResult{}, nil
		},
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodPost, "/api/neoforge/change", strings.NewReader(`{"version":""}`)),
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("empty version status = %d, want 400", recorder.Code)
	}
}
