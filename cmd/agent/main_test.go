package main

import (
	"bytes"
	"log"
	"os"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/agent"
)

type fakeAgent struct {
	cfg         agent.Config
	startCalled bool
}

func (f *fakeAgent) Start() { f.startCalled = true }
func (f *fakeAgent) Stop()  {}

func Test_runAgent_UsesConfigAndCallsStart(t *testing.T) {
	t.Setenv("SERVER_URL", "http://test:1234")
	t.Setenv("POLL_INTERVAL", "1s")
	t.Setenv("REPORT_INTERVAL", "3s")

	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(os.Stdout) })

	fa := &fakeAgent{}
	run(func(cfg agent.Config) interface {
		Start()
		Stop()
	} {
		fa.cfg = cfg
		return fa
	})

	if !fa.startCalled {
		t.Errorf("Start() was not called")
	}
	if fa.cfg.ServerURL != "http://test:1234" {
		t.Errorf("wrong ServerURL: %s", fa.cfg.ServerURL)
	}
	if fa.cfg.PollInterval != 1*time.Second {
		t.Errorf("wrong PollInterval: %v", fa.cfg.PollInterval)
	}
	if fa.cfg.ReportInterval != 3*time.Second {
		t.Errorf("wrong ReportInterval: %v", fa.cfg.ReportInterval)
	}

	if got := buf.String(); got == "" || !bytes.Contains(buf.Bytes(), []byte("agent started")) {
		t.Errorf("expected startup log, got %q", got)
	}
}
