package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fpbcraft/fpbcraft/tools/fpbpack/internal/catalog"
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

func TestRefreshEndpointsRunOnServerContext(t *testing.T) {
	refreshCalled := make(chan struct{}, 1)
	updateCalled := make(chan struct{}, 1)
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()

	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			BackgroundContext: serverCtx,
			Refresh: func(ctx context.Context) error {
				if ctx.Err() != nil {
					t.Fatalf("refresh received cancelled server context: %v", ctx.Err())
				}
				refreshCalled <- struct{}{}
				return nil
			},
			CheckUpdates: func(ctx context.Context) error {
				if ctx.Err() != nil {
					t.Fatalf("update refresh received cancelled server context: %v", ctx.Err())
				}
				updateCalled <- struct{}{}
				return nil
			},
		},
	)

	tests := []struct {
		route string
		called <-chan struct{}
	}{
		{route: "/api/refresh", called: refreshCalled},
		{route: "/api/updates/check", called: updateCalled},
	}
	for _, test := range tests {
		requestCtx, cancelRequest := context.WithCancel(context.Background())
		cancelRequest()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(
			recorder,
			httptest.NewRequest(http.MethodPost, test.route, nil).WithContext(requestCtx),
		)
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("%s code = %d, want 202", test.route, recorder.Code)
		}
		select {
		case <-test.called:
		case <-time.After(time.Second):
			t.Fatalf("%s background refresh did not run", test.route)
		}
	}
}

func TestRefreshEndpointsDeduplicateRunningJob(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Refresh: func(context.Context) error {
				started <- struct{}{}
				<-release
				return nil
			},
		},
	)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/refresh", nil))
	if first.Code != http.StatusAccepted {
		t.Fatalf("first refresh = %d", first.Code)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first refresh did not start")
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/refresh", nil))
	if second.Code != http.StatusAccepted {
		t.Fatalf("second refresh = %d", second.Code)
	}
	if !strings.Contains(second.Body.String(), "already_refreshing") {
		t.Fatalf("second response = %s", second.Body.String())
	}
	close(release)
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

func TestUpdateRuleEndpoints(t *testing.T) {
	rules := map[string]service.UpdateRule{}
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Rules: func() map[string]service.UpdateRule { return rules },
			SetRule: func(key string, rule service.UpdateRule) (service.UpdateRule, error) {
				rules[key] = rule
				return rule, nil
			},
			ClearRule: func(key string) error {
				delete(rules, key)
				return nil
			},
		},
	)

	put := httptest.NewRecorder()
	handler.ServeHTTP(
		put,
		httptest.NewRequest(
			http.MethodPut,
			"/api/update-rules",
			strings.NewReader(`{"key":"modrinth:test","rule":{"ignore_mod":true}}`),
		),
	)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT rule = %d: %s", put.Code, put.Body.String())
	}
	if !rules["modrinth:test"].IgnoreMod {
		t.Fatal("rule was not stored")
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/update-rules", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET rules = %d", get.Code)
	}

	del := httptest.NewRecorder()
	handler.ServeHTTP(
		del,
		httptest.NewRequest(http.MethodDelete, "/api/update-rules?key=modrinth%3Atest", nil),
	)
	if del.Code != http.StatusOK {
		t.Fatalf("DELETE rule = %d: %s", del.Code, del.Body.String())
	}
	if _, ok := rules["modrinth:test"]; ok {
		t.Fatal("rule was not cleared")
	}
}

func TestStatusIncludesRefreshStatus(t *testing.T) {
	now := time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			RefreshStatus: func() service.RefreshStatus {
				return service.RefreshStatus{Refreshing: true, LastSuccess: &now}
			},
		},
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var payload struct {
		Refresh service.RefreshStatus `json:"refresh"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Refresh.Refreshing || payload.Refresh.LastSuccess == nil {
		t.Fatalf("unexpected refresh payload: %+v", payload.Refresh)
	}
}

func TestProvidersEndpoint(t *testing.T) {
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Providers: func() []service.ProviderStatus {
				return []service.ProviderStatus{{ID: "modrinth", Label: "Modrinth", Status: "ready"}}
			},
		},
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/providers", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("providers = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestProviderCredentialEndpointsNeverReturnSecret(t *testing.T) {
	status := service.ProviderStatus{
		ID: "curseforge",
		Label: "CurseForge",
		Status: "ready",
		CredentialConfigurable: true,
		CredentialSource: "saved",
	}
	var received string
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			SetProviderCredential: func(_ context.Context, provider, key string) (service.ProviderStatus, error) {
				if provider != "curseforge" {
					t.Fatalf("provider = %q", provider)
				}
				received = key
				return status, nil
			},
			ClearProviderCredential: func(provider string) (service.ProviderStatus, error) {
				if provider != "curseforge" {
					t.Fatalf("provider = %q", provider)
				}
				return service.ProviderStatus{
					ID: "curseforge",
					Label: "CurseForge",
					Status: "needs_configuration",
					CredentialConfigurable: true,
				}, nil
			},
		},
	)

	put := httptest.NewRecorder()
	handler.ServeHTTP(
		put,
		httptest.NewRequest(
			http.MethodPut,
			"/api/providers/curseforge/credentials",
			strings.NewReader(`{"api_key":"super-secret"}`),
		),
	)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT credential = %d: %s", put.Code, put.Body.String())
	}
	if received != "super-secret" {
		t.Fatalf("received key = %q", received)
	}
	if strings.Contains(put.Body.String(), "super-secret") {
		t.Fatal("provider credential leaked in API response")
	}

	del := httptest.NewRecorder()
	handler.ServeHTTP(
		del,
		httptest.NewRequest(http.MethodDelete, "/api/providers/curseforge/credentials", nil),
	)
	if del.Code != http.StatusOK {
		t.Fatalf("DELETE credential = %d: %s", del.Code, del.Body.String())
	}
}


func TestSlice3OperationalEndpoints(t *testing.T) {
	var appliedID string
	var restoredID string
	var placementPath string
	var manualPlanID string
	var manualCandidate string
	var manualBytes string
	var started, stopped bool
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) {
			return management.Snapshot{Status: management.Status{Mode: "read-only", ReadOnly: true}}, nil
		},
		"dev",
		ServerOptions{
			CraftyStatus: func(context.Context) service.CraftyStatus {
				return service.CraftyStatus{
					Configured: true,
					Connected: true,
					State: "stopped",
					ServerID: "server-1",
				}
			},
			StartServer: func(context.Context) (service.CraftyStatus, error) {
				started = true
				return service.CraftyStatus{Configured: true, Connected: true, State: "running"}, nil
			},
			StopServer: func(context.Context) (service.CraftyStatus, error) {
				stopped = true
				return service.CraftyStatus{Configured: true, Connected: true, State: "stopped"}, nil
			},
			CreatePlacementPlan: func(_ context.Context, path string) (planning.Plan, error) {
				placementPath = path
				return planning.Plan{ID: "plan-aaaaaaaaaaaaaaaa", Status: planning.StatusReady, Verified: true}, nil
			},
			ApplyPlan: func(_ context.Context, id string) (service.ApplyResult, error) {
				appliedID = id
				return service.ApplyResult{PlanID: id, Status: "success"}, nil
			},
			RestoreBackup: func(_ context.Context, id string) (service.RestoreResult, error) {
				restoredID = id
				return service.RestoreResult{BackupID: id, Status: "success"}, nil
			},
			AcceptManualArtifact: func(
				_ context.Context,
				planID string,
				candidateKey string,
				reader io.Reader,
			) (planning.Plan, error) {
				manualPlanID = planID
				manualCandidate = candidateKey
				content, err := io.ReadAll(reader)
				if err != nil {
					return planning.Plan{}, err
				}
				manualBytes = string(content)
				return planning.Plan{
					ID: planID,
					Status: planning.StatusReady,
					Verified: true,
				}, nil
			},
		},
	)

	statusRecorder := httptest.NewRecorder()
	handler.ServeHTTP(statusRecorder, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d", statusRecorder.Code)
	}
	var status struct {
		Mode string `json:"mode"`
		ReadOnly bool `json:"read_only"`
		ServerState string `json:"server_state"`
	}
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Mode != "managed" || status.ReadOnly || status.ServerState != "stopped" {
		t.Fatalf("unexpected managed status: %+v", status)
	}

	for route, flag := range map[string]*bool{
		"/api/server/start": &started,
		"/api/server/stop": &stopped,
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, route, nil))
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("%s = %d: %s", route, recorder.Code, recorder.Body.String())
		}
		if !*flag {
			t.Fatalf("%s did not invoke server control", route)
		}
	}

	placement := httptest.NewRecorder()
	handler.ServeHTTP(
		placement,
		httptest.NewRequest(
			http.MethodPost,
			"/api/placement-plans",
			strings.NewReader("{\"path\":\"mods/example.jar\"}"),
		),
	)
	if placement.Code != http.StatusCreated || placementPath != "mods/example.jar" {
		t.Fatalf("placement plan = %d path=%q body=%s", placement.Code, placementPath, placement.Body.String())
	}

	manual := httptest.NewRecorder()
	handler.ServeHTTP(
		manual,
		httptest.NewRequest(
			http.MethodPost,
			"/api/plans/plan-0123456789abcdef/manual-artifact?candidate_key=curseforge%3A123",
			strings.NewReader("jar-bytes"),
		),
	)
	if manual.Code != http.StatusOK ||
		manualPlanID != "plan-0123456789abcdef" ||
		manualCandidate != "curseforge:123" ||
		manualBytes != "jar-bytes" {
		t.Fatalf(
			"manual artifact = %d plan=%q candidate=%q bytes=%q body=%s",
			manual.Code,
			manualPlanID,
			manualCandidate,
			manualBytes,
			manual.Body.String(),
		)
	}

	apply := httptest.NewRecorder()
	handler.ServeHTTP(
		apply,
		httptest.NewRequest(http.MethodPost, "/api/plans/plan-0123456789abcdef/apply", nil),
	)
	if apply.Code != http.StatusOK || appliedID != "plan-0123456789abcdef" {
		t.Fatalf("apply = %d id=%q body=%s", apply.Code, appliedID, apply.Body.String())
	}

	unconfirmed := httptest.NewRecorder()
	handler.ServeHTTP(
		unconfirmed,
		httptest.NewRequest(
			http.MethodPost,
			"/api/backups/backup-0123456789abcdef/restore",
			strings.NewReader("{\"confirm\":false}"),
		),
	)
	if unconfirmed.Code != http.StatusBadRequest || restoredID != "" {
		t.Fatalf("unconfirmed restore = %d restored=%q", unconfirmed.Code, restoredID)
	}

	confirmed := httptest.NewRecorder()
	handler.ServeHTTP(
		confirmed,
		httptest.NewRequest(
			http.MethodPost,
			"/api/backups/backup-0123456789abcdef/restore",
			strings.NewReader("{\"confirm\":true}"),
		),
	)
	if confirmed.Code != http.StatusOK || restoredID != "backup-0123456789abcdef" {
		t.Fatalf("confirmed restore = %d restored=%q body=%s", confirmed.Code, restoredID, confirmed.Body.String())
	}
}


func TestToolsEndpointsExposeCatalogAndStartInventoryRefresh(t *testing.T) {
	refreshCalled := make(chan struct{}, 1)
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Catalog: func() (catalog.Report, error) {
				return catalog.Report{
					SchemaVersion: catalog.ReportSchemaVersion,
					Managed: []catalog.Entry{{Provider: "modrinth", ProjectID: "test"}},
				}, nil
			},
			CatalogPreview: func() (catalog.Report, error) {
				return catalog.Report{
					SchemaVersion: catalog.ReportSchemaVersion,
					Summary: catalog.Summary{GeneratedProjects: 3, Unresolved: 1},
				}, nil
			},
			RefreshInventory: func(context.Context) error {
				refreshCalled <- struct{}{}
				return nil
			},
		},
	)

	catalogRecorder := httptest.NewRecorder()
	handler.ServeHTTP(catalogRecorder, httptest.NewRequest(http.MethodGet, "/api/catalog", nil))
	if catalogRecorder.Code != http.StatusOK {
		t.Fatalf("catalog = %d: %s", catalogRecorder.Code, catalogRecorder.Body.String())
	}
	var report catalog.Report
	if err := json.Unmarshal(catalogRecorder.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Managed) != 1 || report.Managed[0].ProjectID != "test" {
		t.Fatalf("unexpected catalog payload: %+v", report)
	}

	previewRecorder := httptest.NewRecorder()
	handler.ServeHTTP(
		previewRecorder,
		httptest.NewRequest(http.MethodPost, "/api/catalog/preview", nil),
	)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("catalog preview = %d: %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview catalog.Report
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Summary.GeneratedProjects != 3 || preview.Summary.Unresolved != 1 {
		t.Fatalf("unexpected catalog preview: %+v", preview.Summary)
	}

	refreshRecorder := httptest.NewRecorder()
	handler.ServeHTTP(
		refreshRecorder,
		httptest.NewRequest(http.MethodPost, "/api/inventory/refresh", nil),
	)
	if refreshRecorder.Code != http.StatusAccepted {
		t.Fatalf("inventory refresh = %d: %s", refreshRecorder.Code, refreshRecorder.Body.String())
	}
	select {
	case <-refreshCalled:
	case <-time.After(time.Second):
		t.Fatal("inventory refresh handler did not invoke service")
	}
}


func TestLogsEndpointReturnsStructuredRuntimeEvents(t *testing.T) {
	now := time.Now().UTC()
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Logs: func(limit int) []service.RuntimeLogEntry {
				if limit != 1 {
					t.Fatalf("limit = %d, want 1", limit)
				}
				return []service.RuntimeLogEntry{{
					ID: 7,
					Time: now,
					Level: "error",
					Area: "refresh",
					Message: "provider failed",
				}}
			},
		},
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/logs?limit=1", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("logs = %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	entries, ok := payload["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("unexpected logs payload: %+v", payload)
	}
	entry, ok := entries[0].(map[string]any)
	if !ok || entry["message"] != "provider failed" || entry["area"] != "refresh" {
		t.Fatalf("unexpected log entry: %+v", entries[0])
	}
}


func TestLogsEndpointReturnsBoundedRuntimeEntries(t *testing.T) {
	now := time.Date(2026, 10, 5, 17, 0, 0, 0, time.UTC)
	handler := NewHandlerWithOptions(
		func() (management.Snapshot, error) { return management.Snapshot{}, nil },
		"dev",
		ServerOptions{
			Logs: func(limit int) []service.RuntimeLogEntry {
				if limit != 25 {
					t.Fatalf("limit = %d, want 25", limit)
				}
				return []service.RuntimeLogEntry{{
					ID: 7,
					Time: now,
					Level: "warn",
					Area: "refresh",
					Message: "provider retry",
				}}
			},
		},
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/logs?limit=25", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("logs = %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Entries []service.RuntimeLogEntry `json:"entries"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Entries) != 1 || payload.Entries[0].Message != "provider retry" {
		t.Fatalf("unexpected log payload: %+v", payload.Entries)
	}
}
