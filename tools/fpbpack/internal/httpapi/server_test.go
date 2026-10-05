package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/doctor"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	updatecheck "github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/updates"
)

func TestStatusAndModsEndpointsExposeDomainState(t *testing.T) {
	snapshot := management.Snapshot{
		Inventory: inventory.Inventory{
			GeneratedAt: time.Date(2026, 10, 5, 1, 2, 3, 0, time.UTC),
			Summary:     inventory.Summary{Total: 1, Server: 1},
		},
		Diagnostics: doctor.Report{Summary: doctor.Summary{Warnings: 1}},
		Status: management.Status{
			Mode: "read-only", ReadOnly: true, ServerState: "unknown", Mods: 1,
			Diagnostics: doctor.Summary{Warnings: 1},
		},
		Mods: []management.Mod{{ID: "modrinth:create", Name: "Create", Management: "managed"}},
	}
	handler := NewHandler(func() (management.Snapshot, error) { return snapshot, nil }, "test-version")

	statusRecorder := httptest.NewRecorder()
	handler.ServeHTTP(statusRecorder, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", statusRecorder.Code)
	}
	var status map[string]any
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["version"] != "test-version" || status["mode"] != "read-only" {
		t.Fatalf("unexpected status payload: %#v", status)
	}

	modsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(modsRecorder, httptest.NewRequest(http.MethodGet, "/api/mods", nil))
	if modsRecorder.Code != http.StatusOK {
		t.Fatalf("mods code = %d, want 200", modsRecorder.Code)
	}
	if got := modsRecorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestHandlerReturnsServiceUnavailableWhenSnapshotFails(t *testing.T) {
	handler := NewHandler(func() (management.Snapshot, error) {
		return management.Snapshot{}, errTest
	}, "dev")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", recorder.Code)
	}
}

type testError string

func (e testError) Error() string { return string(e) }

const errTest testError = "test failure"

func TestUpdatesEndpointUsesConfiguredLoader(t *testing.T) {
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Updates: func() (updatecheck.Report, error) {
				return updatecheck.Report{
					Minecraft: "1.21.1",
					Loader:    "neoforge",
					Summary:   updatecheck.Summary{Safe: 2, Review: 1},
				}, nil
			},
		},
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/updates", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", recorder.Code)
	}
	var report updatecheck.Report
	if err := json.Unmarshal(recorder.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Safe != 2 || report.Summary.Review != 1 {
		t.Fatalf("unexpected update report: %+v", report)
	}
}

func TestUpdatesEndpointIsUnavailableWithoutCache(t *testing.T) {
	handler := NewHandler(func() (management.Snapshot, error) { return management.Snapshot{}, nil }, "dev")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/updates", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", recorder.Code)
	}
}

func TestCORSAllowsConfiguredBrowserOrigin(t *testing.T) {
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{AllowedOrigins: []string{"https://fpbcraft.example"}},
	)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "https://fpbcraft.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://fpbcraft.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestCORSDoesNotExposeResponseToUnknownOrigin(t *testing.T) {
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{AllowedOrigins: []string{"https://fpbcraft.example"}},
	)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "https://not-allowed.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected Access-Control-Allow-Origin = %q", got)
	}
}

func TestCORSPreflightSupportsLegacyPrivateNetworkAccess(t *testing.T) {
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{AllowedOrigins: []string{"https://fpbcraft.example"}},
	)

	request := httptest.NewRequest(http.MethodOptions, "/api/status", nil)
	request.Header.Set("Origin", "https://fpbcraft.example")
	request.Header.Set("Access-Control-Request-Method", "GET")
	request.Header.Set("Access-Control-Request-Private-Network", "true")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Private-Network"); got != "true" {
		t.Fatalf("Access-Control-Allow-Private-Network = %q, want true", got)
	}
}

func TestCORSRejectsUnknownOriginPreflight(t *testing.T) {
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{AllowedOrigins: []string{"https://fpbcraft.example"}},
	)

	request := httptest.NewRequest(http.MethodOptions, "/api/status", nil)
	request.Header.Set("Origin", "https://not-allowed.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", recorder.Code)
	}
}
