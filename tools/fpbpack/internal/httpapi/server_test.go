package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/doctor"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/inventory"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/management"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/planning"
	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/service"
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

func TestRefreshEndpointsInvokeServiceCallbacks(t *testing.T) {
	refreshCalls := 0
	updateCalls := 0
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Refresh: func(context.Context) error {
				refreshCalls++
				return nil
			},
			CheckUpdates: func(context.Context) error {
				updateCalls++
				return nil
			},
		},
	)

	for _, route := range []string{"/api/refresh", "/api/updates/check"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, route, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s code = %d, want 200", route, recorder.Code)
		}
	}
	if refreshCalls != 1 || updateCalls != 1 {
		t.Fatalf("refresh calls = %d, update calls = %d", refreshCalls, updateCalls)
	}
}

func TestPlanEndpointsCreateAndReadPlans(t *testing.T) {
	created := planning.Plan{ID: "plan-0123456789abcdef", Status: planning.StatusReady}
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			CreatePlan: func(_ context.Context, keys []string) (planning.Plan, error) {
				if len(keys) != 1 || keys[0] != "modrinth:create" {
					t.Fatalf("unexpected candidate keys: %+v", keys)
				}
				return created, nil
			},
			Plan: func(id string) (planning.Plan, error) {
				if id != created.ID {
					return planning.Plan{}, planning.ErrNotFound
				}
				return created, nil
			},
			Plans: func() ([]planning.Summary, error) {
				return []planning.Summary{created.Summary()}, nil
			},
			History: func() ([]planning.HistoryEvent, error) {
				return []planning.HistoryEvent{created.HistoryEvent()}, nil
			},
		},
	)

	createRecorder := httptest.NewRecorder()
	handler.ServeHTTP(
		createRecorder,
		httptest.NewRequest(http.MethodPost, "/api/plans", strings.NewReader(`{"candidate_keys":["modrinth:create"]}`)),
	)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create code = %d, want 201: %s", createRecorder.Code, createRecorder.Body.String())
	}

	for _, route := range []string{"/api/plans", "/api/plans/" + created.ID, "/api/history"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s code = %d, want 200", route, recorder.Code)
		}
	}
}

func TestRetentionSettingsEndpointsReadAndUpdate(t *testing.T) {
	current := service.RuntimeSettings{RetentionCount: 20}
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Retention: func() service.RuntimeSettings { return current },
			UpdateRetention: func(value service.RuntimeSettings) (service.RuntimeSettings, error) {
				current = value
				return current, nil
			},
		},
	)

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET settings = %d", get.Code)
	}

	put := httptest.NewRecorder()
	handler.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"retention_count":12}`)))
	if put.Code != http.StatusOK {
		t.Fatalf("PUT settings = %d: %s", put.Code, put.Body.String())
	}
	if current.RetentionCount != 12 {
		t.Fatalf("retention = %d, want 12", current.RetentionCount)
	}
}
