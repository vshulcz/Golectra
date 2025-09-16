package agent

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/models"
)

func TestScope_NewRuntimeAgent_Defaults(t *testing.T) {
	ag := NewRuntimeAgent(config.AgentConfig{})

	a, ok := ag.(*runtimeAgent)
	if !ok {
		t.Fatalf("NewRuntimeAgent returned %T; want *runtimeAgent", ag)
	}

	if a.cfg.Address != "http://localhost:8080" {
		t.Errorf("expected default ServerURL, got %s", a.cfg.Address)
	}
	if a.cfg.PollInterval != 2*time.Second {
		t.Errorf("expected default PollInterval=2s, got %v", a.cfg.PollInterval)
	}
	if a.cfg.ReportInterval != 10*time.Second {
		t.Errorf("expected default ReportInterval=10s, got %v", a.cfg.ReportInterval)
	}
}

func TestScope_NewRuntimeAgent_KeepProvidedConfig(t *testing.T) {
	cfg := config.AgentConfig{
		Address:        "http://x:1",
		PollInterval:   1 * time.Second,
		ReportInterval: 3 * time.Second,
	}

	ag := NewRuntimeAgent(cfg)

	a, ok := ag.(*runtimeAgent)
	if !ok {
		t.Fatalf("NewRuntimeAgent returned %T; want *runtimeAgent", ag)
	}

	if a.cfg.Address != "http://x:1" {
		t.Errorf("ServerURL mismatch: %s", a.cfg.Address)
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

	a := &runtimeAgent{cfg: config.AgentConfig{Address: srv.URL}}

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

	a := &runtimeAgent{cfg: config.AgentConfig{Address: srv.URL}}
	err := a.postGauge("X", 1.23)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("expected error about 400, got %v", err)
	}
}

func TestScope_mustJoinURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		path string
		want string
	}{
		{"no-trailing-slash", "http://h:1", "/update/", "http://h:1/update/"},
		{"with-trailing-slash", "http://h:1/", "/update/", "http://h:1/update/"},
		{"with-base-path", "http://h:1/api", "/update/", "http://h:1/api/update/"},
		{"with-base-path-trailing", "http://h:1/api/", "/update/", "http://h:1/api/update/"},
		{"invalid-base", "%%%", "/update/", "%%%/update/"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustJoinURL(tc.base, tc.path); got != tc.want {
				t.Fatalf("mustJoinURL(%q,%q)=%q want %q", tc.base, tc.path, got, tc.want)
			}
		})
	}
}

func TestScope_postJSON(t *testing.T) {
	type seen struct {
		ct   string
		acc  string
		ae   string
		ce   string
		body models.Metrics
	}
	var got seen

	srvOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.ct = r.Header.Get("Content-Type")
		got.acc = r.Header.Get("Accept")
		got.ae = r.Header.Get("Accept-Encoding")
		got.ce = r.Header.Get("Content-Encoding")

		defer r.Body.Close()
		var reader io.Reader = r.Body
		if strings.Contains(strings.ToLower(got.ce), "gzip") {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "bad gzip", http.StatusBadRequest)
				return
			}
			defer gr.Close()
			reader = gr
		}
		b, _ := io.ReadAll(reader)
		json.Unmarshal(b, &got.body)

		w.WriteHeader(http.StatusOK)
	}))
	defer srvOK.Close()

	agt := &runtimeAgent{cfg: config.AgentConfig{Address: srvOK.URL}}

	t.Run("gauge", func(t *testing.T) {
		v := 123.45
		msg := models.Metrics{ID: "Alloc", MType: string(models.Gauge), Value: &v}
		if err := agt.postJSON(srvOK.URL, msg); err != nil {
			t.Fatalf("postJSON gauge err: %v", err)
		}
		if got.ct != "application/json" {
			t.Fatalf("Content-Type=%q want application/json", got.ct)
		}
		if got.acc != "application/json" {
			t.Fatalf("Accept=%q want application/json", got.acc)
		}
		if !strings.Contains(strings.ToLower(got.ae), "gzip") {
			t.Fatalf("Accept-Encoding=%q want to contain gzip", got.ae)
		}
		if strings.ToLower(got.ce) != "gzip" {
			t.Fatalf("Content-Encoding=%q want gzip", got.ce)
		}
		if got.body.ID != "Alloc" || got.body.MType != "gauge" || got.body.Value == nil {
			t.Fatalf("bad body: %+v", got.body)
		}
	})

	t.Run("counter", func(t *testing.T) {
		d := int64(7)
		msg := models.Metrics{ID: "PollCount", MType: string(models.Counter), Delta: &d}
		if err := agt.postJSON(srvOK.URL, msg); err != nil {
			t.Fatalf("postJSON counter err: %v", err)
		}
		if got.body.ID != "PollCount" || got.body.MType != "counter" || got.body.Delta == nil || *got.body.Delta != 7 {
			t.Fatalf("bad body: %+v", got.body)
		}
	})

	t.Run("non-200 returns error", func(t *testing.T) {
		srvBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "bad", http.StatusBadRequest)
		}))
		defer srvBad.Close()
		v := 1.0
		err := agt.postJSON(srvBad.URL, models.Metrics{ID: "X", MType: "gauge", Value: &v})
		if err == nil || !strings.Contains(err.Error(), "400") {
			t.Fatalf("want error with 400, got %v", err)
		}
	})
}

func TestScope_reportOnce_SendsAllMetrics(t *testing.T) {
	var (
		gotPaths []string
		gotCT    []string
		gotMsgs  []models.Metrics
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		gotCT = append(gotCT, r.Header.Get("Content-Type"))

		defer r.Body.Close()
		var reader io.Reader = r.Body
		if strings.Contains(strings.ToLower(r.Header.Get("Content-Encoding")), "gzip") {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "bad gzip", http.StatusBadRequest)
				return
			}
			defer gr.Close()
			reader = gr
		}
		b, _ := io.ReadAll(reader)
		var m models.Metrics
		json.Unmarshal(b, &m)
		gotMsgs = append(gotMsgs, m)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st := newStats()
	st.setGauge("Alloc", 1.23)
	st.addCounter("PollCount", 3)
	agt := &runtimeAgent{
		cfg:   config.AgentConfig{Address: srv.URL},
		stats: st,
	}

	agt.reportOnce()

	if len(gotMsgs) != 2 {
		t.Fatalf("sent %d messages, want 2", len(gotMsgs))
	}
	for _, p := range gotPaths {
		if p != "/update/" {
			t.Fatalf("path=%q want /update/", p)
		}
	}
	if gotCT[0] != "application/json" || gotCT[1] != "application/json" {
		t.Fatalf("content-types=%v want application/json", gotCT)
	}

	var haveAlloc, havePoll bool
	for _, m := range gotMsgs {
		switch m.ID {
		case "Alloc":
			if m.MType != "gauge" || m.Value == nil {
				t.Fatalf("bad gauge: %+v", m)
			}
			haveAlloc = true
		case "PollCount":
			if m.MType != "counter" || m.Delta == nil || *m.Delta != 3 {
				t.Fatalf("bad counter: %+v", m)
			}
			havePoll = true
		}
	}
	if !haveAlloc || !havePoll {
		t.Fatalf("missing metrics: alloc=%v poll=%v", haveAlloc, havePoll)
	}
}
