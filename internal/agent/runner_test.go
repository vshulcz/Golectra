package agent

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/config"
)

func TestWithHTTPClient(t *testing.T) {
	hc := &http.Client{Timeout: 123 * time.Millisecond}
	var o Options
	WithHTTPClient(hc)(&o)

	if o.HTTPClient != hc {
		t.Fatalf("WithHTTPClient did not set client: got=%p want=%p", o.HTTPClient, hc)
	}
}

func TestRun_CtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.AgentConfig{
		Address:        "http://example",
		PollInterval:   2 * time.Millisecond,
		ReportInterval: 10 * time.Second,
	}

	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	time.Sleep(10 * time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context cancel")
	}
}

func TestRun_ImmediateCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := config.AgentConfig{
		Address:        "http://example",
		PollInterval:   1 * time.Millisecond,
		ReportInterval: 1 * time.Second,
	}

	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg) }()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Run() did not return when context was already canceled")
	}
}
