package retry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_SuccessAfterRetry(t *testing.T) {
	var calls int32
	wantErr := errors.New("retry")
	op := func() error {
		if atomic.AddInt32(&calls, 1) == 1 {
			return wantErr
		}
		return nil
	}

	err := Retry(context.Background(), []time.Duration{1 * time.Millisecond}, func(err error) bool {
		return errors.Is(err, wantErr)
	}, op)
	if err != nil {
		t.Fatalf("Retry error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls=%d want 2", got)
	}
}

func TestRetry_NonRetryableStops(t *testing.T) {
	var calls int32
	wantErr := errors.New("nope")
	op := func() error {
		atomic.AddInt32(&calls, 1)
		return wantErr
	}

	err := Retry(context.Background(), []time.Duration{1 * time.Millisecond}, func(error) bool { return false }, op)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Retry error=%v want %v", err, wantErr)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls=%d want 1", got)
	}
}

func TestRetry_ContextCancel(t *testing.T) {
	var calls int32
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Retry(ctx, []time.Duration{1 * time.Millisecond}, func(error) bool { return true }, func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("retry")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Retry error=%v want context canceled", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls=%d want 1", got)
	}
}

func TestRetry_NoDelaysStops(t *testing.T) {
	wantErr := errors.New("no retry")
	err := Retry(context.Background(), nil, func(error) bool { return true }, func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Retry error=%v want %v", err, wantErr)
	}
}

func TestRetry_ContextCancelsDuringDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	err := Retry(ctx, []time.Duration{50 * time.Millisecond}, func(error) bool { return true }, func() error {
		cancel()
		close(done)
		return errors.New("retry")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Retry error=%v want context canceled", err)
	}
}
