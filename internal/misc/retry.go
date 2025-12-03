package misc

import (
	"context"
	"time"
)

// DefaultBackoff provides a simple backoff schedule for retryable operations.
var DefaultBackoff = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	5 * time.Second,
}

// Retry executes op until it succeeds, the context expires, or no retryable error occurs.
func Retry(ctx context.Context, delays []time.Duration, isRetryable func(error) bool, op func() error) error {
	var err error
	for i := 0; ; i++ {
		if err = op(); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if i >= len(delays) || !isRetryable(err) {
			return err
		}
		t := time.NewTimer(delays[i])
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}
