package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/config"
)

type capturedReq struct {
	Method string
	Path   string
	Hdr    http.Header
	Body   map[string]any
}

type captureRT struct {
	mu        sync.Mutex
	reqs      []capturedReq
	replyGzip bool
	status    int
}

func (rt *captureRT) RoundTrip(req *http.Request) (*http.Response, error) {
	var raw []byte
	if strings.Contains(strings.ToLower(req.Header.Get("Content-Encoding")), "gzip") {
		gr, err := gzip.NewReader(req.Body)
		if err != nil {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader("bad gzip")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}
		defer gr.Close()
		raw, _ = io.ReadAll(gr)
	} else {
		raw, _ = io.ReadAll(req.Body)
	}
	req.Body.Close()

	var m map[string]any
	json.Unmarshal(raw, &m)

	rt.mu.Lock()
	rt.reqs = append(rt.reqs, capturedReq{
		Method: req.Method,
		Path:   req.URL.Path,
		Hdr:    req.Header.Clone(),
		Body:   m,
	})
	rt.mu.Unlock()

	h := make(http.Header)
	var body io.ReadCloser = io.NopCloser(strings.NewReader("ok"))
	if rt.replyGzip {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		zw.Write([]byte("ok"))
		zw.Close()
		h.Set("Content-Encoding", "gzip")
		body = io.NopCloser(bytes.NewReader(buf.Bytes()))
	}
	code := rt.status
	if code == 0 {
		code = http.StatusOK
	}
	return &http.Response{
		StatusCode: code,
		Header:     h,
		Body:       body,
		Request:    req,
	}, nil
}

func (rt *captureRT) snapshot() []capturedReq {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := make([]capturedReq, len(rt.reqs))
	copy(out, rt.reqs)
	return out
}

func withHTTPClient(hc *http.Client) Option {
	return func(o *Options) { o.HTTPClient = hc }
}

func TestNew_InvalidAddress(t *testing.T) {
	cfg := config.AgentConfig{
		Address:        "http://%zz",
		PollInterval:   5 * time.Millisecond,
		ReportInterval: 10 * time.Millisecond,
	}
	_, err := New(cfg)
	if err == nil {
		t.Fatalf("expected error from New with bad address")
	}
}

func TestReportOnce(t *testing.T) {
	rt := &captureRT{}
	hc := &http.Client{Transport: rt, Timeout: 2 * time.Second}

	cfg := config.AgentConfig{
		Address:        "http://example",
		PollInterval:   1 * time.Second,
		ReportInterval: 1 * time.Second,
	}

	aIntf, err := New(cfg, withHTTPClient(hc))
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	ra := aIntf.(*runtimeAgent)

	ra.stats.SetGauge("g1", 1.23)
	ra.stats.AddCounter("c1", 7)

	ra.reportOnce()

	reqs := rt.snapshot()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests (1 gauge + 1 counter), got %d", len(reqs))
	}
	for i, r := range reqs {
		if r.Method != http.MethodPost {
			t.Fatalf("req[%d] method=%s want POST", i, r.Method)
		}
		if r.Path != "/update" {
			t.Fatalf("req[%d] path=%q want /update", i, r.Path)
		}
		if ct := r.Hdr.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("req[%d] Content-Type=%q", i, ct)
		}
		if ce := strings.ToLower(r.Hdr.Get("Content-Encoding")); !strings.Contains(ce, "gzip") {
			t.Fatalf("req[%d] Content-Encoding=%q want gzip", i, ce)
		}
	}

	foundGauge := false
	foundCounter := false
	for _, r := range reqs {
		id, _ := r.Body["id"].(string)
		typ, _ := r.Body["type"].(string)
		switch typ {
		case "gauge":
			if id == "g1" {
				v, _ := r.Body["value"].(float64)
				if v != 1.23 {
					t.Fatalf("gauge value=%v want 1.23", v)
				}
				if _, ok := r.Body["delta"]; ok {
					t.Fatalf("gauge must not contain delta")
				}
				foundGauge = true
			}
		case "counter":
			if id == "c1" {
				raw := r.Body["delta"]
				var d int64
				switch vv := raw.(type) {
				case float64:
					d = int64(vv)
				case json.Number:
					i64, _ := vv.Int64()
					d = i64
				}
				if d != 7 {
					t.Fatalf("counter delta=%v want 7", raw)
				}
				if _, ok := r.Body["value"]; ok {
					t.Fatalf("counter must not contain value")
				}
				foundCounter = true
			}
		default:
			t.Fatalf("unexpected type %q", typ)
		}
	}
	if !foundGauge || !foundCounter {
		t.Fatalf("did not capture both gauge and counter: gauge=%v counter=%v", foundGauge, foundCounter)
	}
}

func TestStartStop(t *testing.T) {
	rt := &captureRT{}
	hc := &http.Client{Transport: rt, Timeout: 2 * time.Second}

	cfg := config.AgentConfig{
		Address:        "http://example",
		PollInterval:   5 * time.Millisecond,
		ReportInterval: 12 * time.Millisecond,
	}

	aIntf, err := New(cfg, withHTTPClient(hc))
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	ra := aIntf.(*runtimeAgent)

	go ra.Start()
	ok := waitUntil(500*time.Millisecond, func() bool {
		for _, r := range rt.snapshot() {
			if id, _ := r.Body["id"].(string); id == "PollCount" {
				return true
			}
		}
		return false
	})
	if !ok {
		ra.Stop()
		t.Fatalf("did not observe PollCount send within timeout")
	}

	before := len(rt.snapshot())

	ra.Stop()
	time.Sleep(3 * cfg.ReportInterval)

	after := len(rt.snapshot())
	if after > before+1 {
		t.Fatalf("requests keep growing after Stop(): before=%d after=%d", before, after)
	}
}

func waitUntil(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(1 * time.Millisecond)
	}
	return false
}
