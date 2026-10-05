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


func TestWebHandlerIsServedWithoutShadowingAPI(t *testing.T) {
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Web: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("web:" + r.URL.Path))
			}),
		},
	)

	webRecorder := httptest.NewRecorder()
	handler.ServeHTTP(webRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if webRecorder.Code != http.StatusOK || webRecorder.Body.String() != "web:/" {
		t.Fatalf("unexpected web response: code=%d body=%q", webRecorder.Code, webRecorder.Body.String())
	}

	apiRecorder := httptest.NewRecorder()
	handler.ServeHTTP(apiRecorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if apiRecorder.Code != http.StatusOK {
		t.Fatalf("health code = %d, want 200", apiRecorder.Code)
	}
	if apiRecorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("health endpoint was shadowed by web handler")
	}
}
