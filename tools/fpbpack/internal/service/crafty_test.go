package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type testCraftyState struct {
	mu      sync.Mutex
	running bool
}

func newTestCraftyServer(t *testing.T, initialRunning bool) (*httptest.Server, *testCraftyState) {
	t.Helper()
	state := &testCraftyState{running: initialRunning}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/servers/server-1/stats":
			state.mu.Lock()
			running := state.running
			state.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ok",
				"data": map[string]any{"running": running},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/servers/server-1/action/stop_server":
			state.mu.Lock()
			state.running = false
			state.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": map[string]any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/servers/server-1/action/start_server":
			state.mu.Lock()
			state.running = true
			state.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, state
}

func TestCraftyStatusAndExplicitControls(t *testing.T) {
	server, _ := newTestCraftyServer(t, true)
	service := &Service{
		options: Options{
			CraftyURL:      server.URL,
			CraftyServerID: "server-1",
			CraftyToken:    "test-token",
		},
	}

	status := service.CraftyStatus(context.Background())
	if !status.Configured || !status.Connected || status.State != "running" {
		t.Fatalf("unexpected initial Crafty status: %+v", status)
	}

	status, err := service.StopServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "stopped" {
		t.Fatalf("stop returned state %q", status.State)
	}
	if err := service.requireServerStopped(context.Background()); err != nil {
		t.Fatalf("stopped server was rejected: %v", err)
	}

	status, err = service.StartServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "running" {
		t.Fatalf("start returned state %q", status.State)
	}
	if err := service.requireServerStopped(context.Background()); err == nil {
		t.Fatal("running server should block mutation")
	}
}

func TestRequireServerStoppedFailsClosedWithoutCrafty(t *testing.T) {
	service := &Service{}
	if err := service.requireServerStopped(context.Background()); err == nil {
		t.Fatal("unconfigured Crafty must not be treated as stopped")
	}
}
