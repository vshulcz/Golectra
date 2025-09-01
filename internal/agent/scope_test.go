package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScope_NewRuntimeAgent_Defaults(t *testing.T) {
	a := NewRuntimeAgent(Config{})

	if a.cfg.ServerURL != "http://localhost:8080" {
		t.Errorf("expected default ServerURL, got %s", a.cfg.ServerURL)
	}
	if a.cfg.PollInterval != 2*time.Second {
		t.Errorf("expected default PollInterval=2s, got %v", a.cfg.PollInterval)
	}
	if a.cfg.ReportInterval != 10*time.Second {
		t.Errorf("expected default ReportInterval=10s, got %v", a.cfg.ReportInterval)
	}
}

func TestScope_NewRuntimeAgent_KeepProvidedConfig(t *testing.T) {
	cfg := Config{
		ServerURL:      "http://x:1",
		PollInterval:   1 * time.Second,
		ReportInterval: 3 * time.Second,
	}
	a := NewRuntimeAgent(cfg)

	if a.cfg.ServerURL != "http://x:1" {
		t.Errorf("ServerURL mismatch: %s", a.cfg.ServerURL)
	}
	if a.cfg.PollInterval != 1*time.Second {
		t.Errorf("PollInterval mismatch: %v", a.cfg.PollInterval)
	}
	if a.cfg.ReportInterval != 3*time.Second {
		t.Errorf("ReportInterval mismatch: %v", a.cfg.ReportInterval)
	}
}

func TestScope_RuntimeAgent_postGaugeAndCounter(t *testing.T) {
	var lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &runtimeAgent{cfg: Config{ServerURL: srv.URL}}

	if err := a.postGauge("Alloc", 123.4); err != nil {
		t.Fatalf("postGauge error: %v", err)
	}
	if !strings.Contains(lastPath, "/update/gauge/Alloc/123.4") {
		t.Errorf("unexpected path: %s", lastPath)
	}

	if err := a.postCounter("PollCount", 7); err != nil {
		t.Fatalf("postCounter error: %v", err)
	}
	if !strings.Contains(lastPath, "/update/counter/PollCount/7") {
		t.Errorf("unexpected path: %s", lastPath)
	}
}

func TestScope_RuntimeAgent_post_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer srv.Close()

	a := &runtimeAgent{cfg: Config{ServerURL: srv.URL}}
	err := a.postGauge("X", 1.23)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("expected error about 400, got %v", err)
	}
}
