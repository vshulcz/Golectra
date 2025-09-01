package main

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/agent"
)

func TestRun_UsesEnvAndCallsStart(t *testing.T) {
	t.Setenv("SERVER_URL", "http://abc:123")
	t.Setenv("POLL_INTERVAL", "1s")
	t.Setenv("REPORT_INTERVAL", "5s")

	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(os.Stdout) })

	var gotCfg agent.Config
	run(func(cfg agent.Config) interface {
		Start()
		Stop()
	} {
		gotCfg = cfg
		return &fakeAgent{}
	})

	if gotCfg.ServerURL != "http://abc:123" {
		t.Errorf("wrong ServerURL: %s", gotCfg.ServerURL)
	}
	if gotCfg.PollInterval != 1*time.Second {
		t.Errorf("wrong PollInterval: %v", gotCfg.PollInterval)
	}
	if gotCfg.ReportInterval != 5*time.Second {
		t.Errorf("wrong ReportInterval: %v", gotCfg.ReportInterval)
	}

	if !strings.Contains(buf.String(), "agent started") {
		t.Errorf("expected log to contain 'agent started', got %q", buf.String())
	}
}
