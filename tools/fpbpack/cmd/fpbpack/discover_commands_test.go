package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPeriodicRefreshDoesNotRunBeforeFirstTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticks := make(chan time.Time)
	var calls atomic.Int32
	called := make(chan struct{}, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		periodicRefreshLoop(
			ctx,
			ticks,
			func(context.Context) error {
				calls.Add(1)
				called <- struct{}{}
				return nil
			},
			nil,
		)
	}()

	if calls.Load() != 0 {
		t.Fatalf("refresh called before a periodic tick: %d", calls.Load())
	}

	ticks <- time.Now()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("refresh was not called after periodic tick")
	}
	if calls.Load() != 1 {
		t.Fatalf("refresh calls = %d, want 1", calls.Load())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("periodic refresh loop did not stop")
	}
}
