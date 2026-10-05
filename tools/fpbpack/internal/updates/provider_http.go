package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ProviderHTTPError struct {
	Provider string
	Status   int
	Message  string
}

func (e *ProviderHTTPError) Error() string {
	return fmt.Sprintf("%s returned HTTP %d: %s", e.Provider, e.Status, e.Message)
}

type RefreshMode string

const (
	RefreshModeBackground  RefreshMode = "background"
	RefreshModeInteractive RefreshMode = "interactive"
)

type requestPacer struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func (p *requestPacer) wait(ctx context.Context) error {
	if p == nil || p.interval <= 0 {
		return nil
	}
	p.mu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	if now.Before(p.next) {
		wait = p.next.Sub(now)
	}
	base := now
	if p.next.After(base) {
		base = p.next
	}
	p.next = base.Add(p.interval)
	p.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var providerPacers = map[string]map[RefreshMode]*requestPacer{
	"modrinth": {
		RefreshModeBackground:  {interval: 500 * time.Millisecond},
		RefreshModeInteractive: {interval: 125 * time.Millisecond},
	},
	"curseforge": {
		RefreshModeBackground:  {interval: 750 * time.Millisecond},
		RefreshModeInteractive: {interval: 200 * time.Millisecond},
	},
	"github": {
		RefreshModeBackground:  {interval: 2 * time.Second},
		RefreshModeInteractive: {interval: 500 * time.Millisecond},
	},
}

func normalizeRefreshMode(mode RefreshMode) RefreshMode {
	if mode == RefreshModeInteractive {
		return RefreshModeInteractive
	}
	return RefreshModeBackground
}

func providerPacer(provider string, mode RefreshMode) *requestPacer {
	provider = strings.ToLower(strings.TrimSpace(provider))
	mode = normalizeRefreshMode(mode)
	if byMode, ok := providerPacers[provider]; ok {
		return byMode[mode]
	}
	return nil
}

func providerConcurrency(mode RefreshMode) int {
	if normalizeRefreshMode(mode) == RefreshModeInteractive {
		return 6
	}
	return 2
}

func doJSONWithRetry(
	ctx context.Context,
	provider string,
	mode RefreshMode,
	client *http.Client,
	requestFactory func() (*http.Request, error),
	target any,
) error {
	const maxAttempts = 5
	pacer := providerPacer(provider, mode)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := pacer.wait(ctx); err != nil {
			return err
		}
		request, err := requestFactory()
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			if attempt == maxAttempts-1 {
				return err
			}
			if err := waitForRetry(ctx, retryDelay(nil, attempt)); err != nil {
				return err
			}
			continue
		}

		if response.StatusCode == http.StatusOK {
			const maxJSONBytes = 32 << 20
			body, readErr := io.ReadAll(io.LimitReader(response.Body, maxJSONBytes+1))
			closeErr := response.Body.Close()
			if readErr != nil {
				if attempt < maxAttempts-1 {
					if err := waitForRetry(ctx, retryDelay(nil, attempt)); err != nil {
						return err
					}
					continue
				}
				return readErr
			}
			if closeErr != nil {
				if attempt < maxAttempts-1 {
					if err := waitForRetry(ctx, retryDelay(nil, attempt)); err != nil {
						return err
					}
					continue
				}
				return closeErr
			}
			if len(body) > maxJSONBytes {
				return fmt.Errorf("%s response exceeded %d byte JSON safety limit", provider, maxJSONBytes)
			}
			decodeErr := decodeJSONAtomically(body, target)
			if decodeErr != nil {
				if retryableJSONDecodeError(decodeErr) && attempt < maxAttempts-1 {
					if err := waitForRetry(ctx, retryDelay(nil, attempt)); err != nil {
						return err
					}
					continue
				}
				return decodeErr
			}
			return nil
		}

		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		if !retryableProviderResponse(provider, response.StatusCode, response.Header) || attempt == maxAttempts-1 {
			return &ProviderHTTPError{
				Provider: provider,
				Status: response.StatusCode,
				Message: strings.TrimSpace(string(message)),
			}
		}
		if err := waitForRetry(ctx, retryDelay(response.Header, attempt)); err != nil {
			return err
		}
	}
	return fmt.Errorf("%s request failed after retries", provider)
}

func decodeJSONAtomically(body []byte, target any) error {
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return json.Unmarshal(body, target)
	}
	temporary := reflect.New(value.Elem().Type())
	if err := json.Unmarshal(body, temporary.Interface()); err != nil {
		return err
	}
	value.Elem().Set(temporary.Elem())
	return nil
}

func retryableJSONDecodeError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unexpected eof") ||
		strings.Contains(message, "unexpected end of json input")
}

func retryableProviderResponse(provider string, status int, header http.Header) bool {
	if status == http.StatusTooManyRequests || status == http.StatusRequestTimeout {
		return true
	}
	if status >= 500 && status <= 599 {
		return true
	}
	if provider == "github" && status == http.StatusForbidden {
		if strings.TrimSpace(header.Get("X-RateLimit-Remaining")) == "0" {
			return true
		}
	}
	return false
}

func retryDelay(header http.Header, attempt int) time.Duration {
	if header != nil {
		if value := strings.TrimSpace(header.Get("Retry-After")); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
				return time.Duration(seconds) * time.Second
			}
			if when, err := http.ParseTime(value); err == nil {
				if wait := time.Until(when); wait > 0 {
					return wait
				}
			}
		}
		if reset := strings.TrimSpace(header.Get("X-RateLimit-Reset")); reset != "" {
			if unix, err := strconv.ParseInt(reset, 10, 64); err == nil {
				if wait := time.Until(time.Unix(unix, 0)); wait > 0 {
					return wait
				}
			}
		}
	}
	base := time.Second << min(attempt, 4)
	if base > 20*time.Second {
		return 20 * time.Second
	}
	return base
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
