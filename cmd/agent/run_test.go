package main

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/agent"
	"github.com/vshulcz/Golectra/internal/config"
)

type fakeAgent struct{}

func (f *fakeAgent) Start() {}
func (f *fakeAgent) Stop()  {}

func TestRun_UsesEnvAndCallsStart(t *testing.T) {
	t.Setenv("ADDRESS", "http://abc:123")
	t.Setenv("POLL_INTERVAL", "1s")
	t.Setenv("REPORT_INTERVAL", "5s")

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"agent"}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(os.Stdout) })

	var gotCfg config.AgentConfig
	run(func(cfg config.AgentConfig) agent.Agent {
		gotCfg = cfg
		return &fakeAgent{}
	})

	if gotCfg.Address != "http://abc:123" {
		t.Errorf("wrong Address: %s", gotCfg.Address)
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

func TestRun_FlagsOverrideEnv(t *testing.T) {
	t.Setenv("ADDRESS", "http://env:9090")
	t.Setenv("POLL_INTERVAL", "10s")
	t.Setenv("REPORT_INTERVAL", "20s")

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"agent", "-a", ":8081", "-p", "1", "-r", "5"}

	var gotCfg config.AgentConfig
	run(func(cfg config.AgentConfig) agent.Agent {
		gotCfg = cfg
		return &fakeAgent{}
	})

	if gotCfg.Address != "http://localhost:8081" {
		t.Errorf("wrong Address: %s", gotCfg.Address)
	}
	if gotCfg.PollInterval != 1*time.Second {
		t.Errorf("wrong PollInterval: %v", gotCfg.PollInterval)
	}
	if gotCfg.ReportInterval != 5*time.Second {
		t.Errorf("wrong ReportInterval: %v", gotCfg.ReportInterval)
	}
}
