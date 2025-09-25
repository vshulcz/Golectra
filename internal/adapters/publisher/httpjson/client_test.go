package httpjson

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
)

func TestNew_NormalizeBaseAndTimeout(t *testing.T) {
	tests := []struct {
		name  string
		addr  string
		want  string
		nilHC bool
	}{
		{"no_scheme_host_port", "localhost:8080", "http://localhost:8080", true},
		{"http_scheme", "http://example.com:9000", "http://example.com:9000", true},
		{"https_scheme", "https://api.local", "https://api.local", true},
		{"trailing_slash_trim", "http://x:1/", "http://x:1", true},
		{"with_path_kept", "http://x:1/base", "http://x:1/base", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var hc *http.Client
			if !tc.nilHC {
				hc = &http.Client{}
			}
			c, err := New(tc.addr, hc)
			if err != nil {
				t.Fatalf("New error: %v", err)
			}
			if got := c.base.String(); got != tc.want {
				t.Fatalf("base=%q want %q", got, tc.want)
			}
			if tc.nilHC {
				if c.hc == nil || c.hc.Timeout != 10*time.Second {
					t.Fatalf("default http.Client timeout = %v, want 10s", c.hc.Timeout)
				}
			}
		})
	}
}

func Test_normalizeBase(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"localhost:8080", "http://localhost:8080"},
		{"http://x:1/", "http://x:1"},
		{"https://x:1////", "https://x:1"},
		{"http://x:1/base", "http://x:1/base"},
	}
	for _, tc := range tests {
		if got := normalizeBase(tc.in); got != tc.want {
			t.Fatalf("normalizeBase(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestNew_InvalidURL(t *testing.T) {
	_, err := New("http://%zz", nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestClient_JoinPath(t *testing.T) {
	c, err := New("http://x:1/base", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.endpoint("/update"); got != "http://x:1/base/update" {
		t.Fatalf("endpoint=%q want %q", got, "http://x:1/base/update")
	}

	c2, _ := New("http://x:1/base/", nil)
	if got := c2.endpoint("/update"); got != "http://x:1/base/update" {
		t.Fatalf("endpoint=%q want %q", got, "http://x:1/base/update")
	}
}

func TestSendOne_VariousResponses(t *testing.T) {
	type recv struct {
		metric domain.Metrics
		method string
		path   string
		ct     string
		ce     string
		ae     string
		aa     string
	}

	tests := []struct {
		name        string
		serverReply func(w http.ResponseWriter, r *http.Request)
		wantErr     string
	}{
		{
			name: "plain_200_ok",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			},
		},
		{
			name: "gzip_200_ok",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				zw := gzip.NewWriter(w)
				_, _ = zw.Write([]byte("ok"))
				_ = zw.Close()
			},
		},
		{
			name: "gzip_header_but_plain_body_should_error",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("not gzipped"))
			},
			wantErr: "bad gzip",
		},
		{
			name: "status_400_should_error",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "bad", http.StatusBadRequest)
			},
			wantErr: "400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got recv
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got.method = r.Method
				got.path = r.URL.Path
				got.ct = r.Header.Get("Content-Type")
				got.ce = r.Header.Get("Content-Encoding")
				got.ae = r.Header.Get("Accept-Encoding")
				got.aa = r.Header.Get("Accept")

				if !strings.HasPrefix(got.ct, "application/json") {
					t.Errorf("Content-Type=%q want application/json", got.ct)
				}
				if !strings.Contains(strings.ToLower(got.ce), "gzip") {
					t.Errorf("Content-Encoding=%q want contains gzip", got.ce)
				}
				if !strings.Contains(strings.ToLower(got.ae), "gzip") {
					t.Errorf("Accept-Encoding=%q want contains gzip", got.ae)
				}
				if !strings.Contains(strings.ToLower(got.aa), "application/json") {
					t.Errorf("Accept=%q want application/json", got.aa)
				}

				gr, err := gzip.NewReader(r.Body)
				if err != nil {
					t.Fatalf("request body not gzipped: %v", err)
				}
				defer gr.Close()
				raw, _ := io.ReadAll(gr)
				if err := json.Unmarshal(raw, &got.metric); err != nil {
					t.Fatalf("bad json: %v; body=%q", err, string(raw))
				}

				tt.serverReply(w, r)
			}))
			defer srv.Close()

			c, err := New(srv.URL, &http.Client{Timeout: 2 * time.Second})
			if err != nil {
				t.Fatal(err)
			}

			val := 123.45
			err = c.SendOne(context.TODO(), domain.Metrics{ID: "Alloc", MType: "gauge", Value: &val})
			if tt.wantErr == "" && err != nil {
				t.Fatalf("SendOne error: %v", err)
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("SendOne err=%v want contains %q", err, tt.wantErr)
				}
				return
			}

			if got.method != http.MethodPost {
				t.Fatalf("method=%s want POST", got.method)
			}
			if got.path != "/update" {
				t.Fatalf("path=%q want /update", got.path)
			}

			if got.metric.ID != "Alloc" || got.metric.MType != "gauge" {
				t.Fatalf("metric=%+v want id=Alloc type=gauge", got.metric)
			}
			if got.metric.Value == nil || *got.metric.Value != 123.45 {
				t.Fatalf("metric.Value=%v want 123.45", got.metric.Value)
			}
			if got.metric.Delta != nil {
				t.Fatal("metric.Delta must be nil for gauge")
			}
		})
	}
}

func TestSendOne_CounterPayloadAndHeaders(t *testing.T) {
	var captured domain.Metrics
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s want POST", r.Method)
		}
		if r.URL.Path != "/update" {
			t.Errorf("path=%q want /update", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("Content-Type=%q", ct)
		}
		if ce := r.Header.Get("Content-Encoding"); !strings.Contains(strings.ToLower(ce), "gzip") {
			t.Errorf("Content-Encoding=%q", ce)
		}
		if ae := r.Header.Get("Accept-Encoding"); !strings.Contains(strings.ToLower(ae), "gzip") {
			t.Errorf("Accept-Encoding=%q", ae)
		}
		if aa := r.Header.Get("Accept"); !strings.Contains(strings.ToLower(aa), "application/json") {
			t.Errorf("Accept=%q", aa)
		}

		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("request not gzipped: %v", err)
		}
		defer gr.Close()
		raw, _ := io.ReadAll(gr)
		if err := json.Unmarshal(raw, &captured); err != nil {
			t.Fatalf("bad json: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New(srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	d := int64(7)
	if err := c.SendOne(context.TODO(), domain.Metrics{ID: "PollCount", MType: "counter", Delta: &d}); err != nil {
		t.Fatalf("SendOne error: %v", err)
	}

	if captured.ID != "PollCount" || captured.MType != "counter" {
		t.Fatalf("metric=%+v want id=PollCount type=counter", captured)
	}
	if captured.Delta == nil || *captured.Delta != 7 {
		t.Fatalf("Delta=%v want 7", captured.Delta)
	}
	if captured.Value != nil {
		t.Fatal("Value must be nil for counter")
	}
}

func TestSendBatch_VariousResponses(t *testing.T) {
	type recv struct {
		method  string
		path    string
		ct      string
		ce      string
		ae      string
		aa      string
		metrics []domain.Metrics
	}

	tests := []struct {
		name        string
		serverReply func(w http.ResponseWriter, r *http.Request)
		wantErrSub  string
	}{
		{
			name: "plain_200_ok",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			},
		},
		{
			name: "gzip_200_ok",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				zw := gzip.NewWriter(w)
				_, _ = zw.Write([]byte("ok"))
				_ = zw.Close()
			},
		},
		{
			name: "gzip_header_but_plain_body_should_error",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("not gzipped"))
			},
			wantErrSub: "bad gzip",
		},
		{
			name: "status_400_should_error",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "bad", http.StatusBadRequest)
			},
			wantErrSub: "400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got recv
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got.method = r.Method
				got.path = r.URL.Path
				got.ct = r.Header.Get("Content-Type")
				got.ce = r.Header.Get("Content-Encoding")
				got.ae = r.Header.Get("Accept-Encoding")
				got.aa = r.Header.Get("Accept")

				if !strings.HasPrefix(got.ct, "application/json") {
					t.Errorf("Content-Type=%q want application/json", got.ct)
				}
				if !strings.Contains(strings.ToLower(got.ce), "gzip") {
					t.Errorf("Content-Encoding=%q want contains gzip", got.ce)
				}
				if !strings.Contains(strings.ToLower(got.ae), "gzip") {
					t.Errorf("Accept-Encoding=%q want contains gzip", got.ae)
				}
				if !strings.Contains(strings.ToLower(got.aa), "application/json") {
					t.Errorf("Accept=%q want application/json", got.aa)
				}

				gr, err := gzip.NewReader(r.Body)
				if err != nil {
					t.Fatalf("request body not gzipped: %v", err)
				}
				defer gr.Close()
				raw, _ := io.ReadAll(gr)
				if err := json.Unmarshal(raw, &got.metrics); err != nil {
					t.Fatalf("bad json: %v; body=%q", err, string(raw))
				}

				tt.serverReply(w, r)
			}))
			defer srv.Close()

			c, err := New(srv.URL, &http.Client{Timeout: 2 * time.Second})
			if err != nil {
				t.Fatal(err)
			}

			val := 1.23
			delta := int64(7)
			err = c.SendBatch(context.TODO(), []domain.Metrics{
				{ID: "Alloc", MType: "gauge", Value: &val},
				{ID: "PollCount", MType: "counter", Delta: &delta},
			})

			if tt.wantErrSub == "" && err != nil {
				t.Fatalf("SendBatch error: %v", err)
			}
			if tt.wantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("SendBatch err=%v want contains %q", err, tt.wantErrSub)
				}
				return
			}

			if got.method != http.MethodPost {
				t.Fatalf("method=%s want POST", got.method)
			}
			if got.path != "/updates" {
				t.Fatalf("path=%q want /updates", got.path)
			}
			if len(got.metrics) != 2 {
				t.Fatalf("metrics len=%d want 2", len(got.metrics))
			}

			var seenGauge, seenCounter bool
			for _, m := range got.metrics {
				switch m.MType {
				case "gauge":
					if m.ID != "Alloc" {
						t.Fatalf("gauge id=%q want Alloc", m.ID)
					}
					if m.Value == nil || *m.Value != 1.23 {
						t.Fatalf("gauge value=%v want 1.23", m.Value)
					}
					if m.Delta != nil {
						t.Fatal("gauge must not contain delta")
					}
					seenGauge = true
				case "counter":
					if m.ID != "PollCount" {
						t.Fatalf("counter id=%q want PollCount", m.ID)
					}
					if m.Delta == nil || *m.Delta != 7 {
						t.Fatalf("counter delta=%v want 7", m.Delta)
					}
					if m.Value != nil {
						t.Fatal("counter must not contain value")
					}
					seenCounter = true
				default:
					t.Fatalf("unexpected metric type %q", m.MType)
				}
			}
			if !seenGauge || !seenCounter {
				t.Fatalf("did not see both gauge and counter: gauge=%v counter=%v", seenGauge, seenCounter)
			}
		})
	}
}

type panicRT struct{}

func (panicRT) RoundTrip(*http.Request) (*http.Response, error) {
	panic("RoundTrip must not be called for empty batch")
}

func TestSendBatch_EmptyBatchIsNoop(t *testing.T) {
	hc := &http.Client{Transport: panicRT{}, Timeout: time.Second}
	c, err := New("http://example", hc)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.SendBatch(context.TODO(), nil); err != nil {
		t.Fatalf("nil batch should be noop, err=%v", err)
	}
	if err := c.SendBatch(context.TODO(), []domain.Metrics{}); err != nil {
		t.Fatalf("empty batch should be noop, err=%v", err)
	}
}
