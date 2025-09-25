package misc

import (
	"context"
	"errors"
	"testing"
	"time"
)

var (
	errRetriable = errors.New("retriable")
	errPermanent = errors.New("permanent")
)

func isRetriable(err error) bool {
	return errors.Is(err, errRetriable)
}

func makeOp(steps []error) (func() error, *int) {
	attempt := 0
	return func() error {
		defer func() { attempt++ }()
		idx := attempt
		if idx >= len(steps) {
			idx = len(steps) - 1
		}
		return steps[idx]
	}, &attempt
}

func TestRetry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		delays       []time.Duration
		steps        []error
		timeout      time.Duration
		cancelBefore bool
		wantAttempts int
		wantErrCheck func(error) bool
	}{
		{
			name:         "success_immediate",
			delays:       []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond},
			steps:        []error{nil},
			wantAttempts: 1,
			wantErrCheck: func(err error) bool { return err == nil },
		},
		{
			name:         "non_retryable_immediate",
			delays:       []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond},
			steps:        []error{errPermanent},
			wantAttempts: 1,
			wantErrCheck: func(err error) bool { return errors.Is(err, errPermanent) },
		},
		{
			name:         "success_after_two_retries",
			delays:       []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond},
			steps:        []error{errRetriable, errRetriable, nil},
			wantAttempts: 3,
			wantErrCheck: func(err error) bool { return err == nil },
		},
		{
			name:         "exhausted_retries_returns_last_error",
			delays:       []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond},
			steps:        []error{errRetriable},
			wantAttempts: 4,
			wantErrCheck: func(err error) bool { return errors.Is(err, errRetriable) },
		},
		{
			name:         "stops_on_permanent_midway",
			delays:       []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond},
			steps:        []error{errRetriable, errPermanent, errRetriable},
			wantAttempts: 2,
			wantErrCheck: func(err error) bool { return errors.Is(err, errPermanent) },
		},
		{
			name:         "context_timeout_during_backoff",
			delays:       []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 50 * time.Millisecond},
			steps:        []error{errRetriable},
			timeout:      10 * time.Millisecond,
			wantAttempts: 1,
			wantErrCheck: func(err error) bool { return errors.Is(err, context.DeadlineExceeded) },
		},
		{
			name:         "context_canceled_before_start",
			delays:       []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 50 * time.Millisecond},
			steps:        []error{errRetriable},
			cancelBefore: true,
			wantAttempts: 1,
			wantErrCheck: func(err error) bool { return errors.Is(err, context.Canceled) },
		},
		{
			name:         "no_delays_means_no_retries",
			delays:       nil,
			steps:        []error{errRetriable},
			wantAttempts: 1,
			wantErrCheck: func(err error) bool { return errors.Is(err, errRetriable) },
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			var cancel context.CancelFunc
			if tc.timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, tc.timeout)
				defer cancel()
			}
			if tc.cancelBefore {
				var c context.CancelFunc
				ctx, c = context.WithCancel(ctx)
				c()
			}

			op, attempts := makeOp(tc.steps)
			err := Retry(ctx, tc.delays, isRetriable, op)

			if !tc.wantErrCheck(err) {
				t.Fatalf("unexpected error: %v", err)
			}
			if *attempts != tc.wantAttempts {
				t.Fatalf("attempts=%d want %d", *attempts, tc.wantAttempts)
			}
		})
	}
}
