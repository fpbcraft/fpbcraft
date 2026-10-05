package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestProviderRetryHonorsRetryAfter(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	var payload map[string]string
	err := doJSONWithRetry(
		context.Background(),
		"test-provider",
		RefreshModeInteractive,
		server.Client(),
		func() (*http.Request, error) {
			return http.NewRequest(http.MethodGet, server.URL, nil)
		},
		&payload,
	)
	if err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	if payload["status"] != "ok" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestRetryDelayUsesProviderReset(t *testing.T) {
	reset := time.Now().Add(2 * time.Second).Unix()
	header := http.Header{}
	header.Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
	delay := retryDelay(header, 0)
	if delay <= 0 || delay > 3*time.Second {
		t.Fatalf("retry delay = %s", delay)
	}
}

func TestProviderConcurrencySeparatesBackgroundAndInteractive(t *testing.T) {
	if providerConcurrency(RefreshModeBackground) >= providerConcurrency(RefreshModeInteractive) {
		t.Fatalf(
			"background concurrency %d should be lower than interactive %d",
			providerConcurrency(RefreshModeBackground),
			providerConcurrency(RefreshModeInteractive),
		)
	}
}
