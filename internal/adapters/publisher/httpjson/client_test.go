package httpjson

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/misc"
)

const (
	updatePath  = "/update"
	updatesPath = "/updates"
)

func mustWrite(t *testing.T, w io.Writer, data []byte) {
	t.Helper()
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func mustClose(t *testing.T, c io.Closer) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func mustReadAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return b
}

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
			c, err := New(tc.addr, hc, "")
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
	_, err := New("http://%zz", nil, "")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestClient_JoinPath(t *testing.T) {
	c, err := New("http://x:1/base", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.endpoint(updatePath); got != "http://x:1/base/update" {
		t.Fatalf("endpoint=%q want %q", got, "http://x:1/base/update")
	}

	c2, _ := New("http://x:1/base/", nil, "")
	if got := c2.endpoint(updatePath); got != "http://x:1/base/update" {
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
				mustWrite(t, w, []byte("ok"))
			},
		},
		{
			name: "gzip_200_ok",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				zw := gzip.NewWriter(w)
				mustWrite(t, zw, []byte("ok"))
				mustClose(t, zw)
			},
		},
		{
			name: "gzip_header_but_plain_body_should_error",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				w.WriteHeader(http.StatusOK)
				mustWrite(t, w, []byte("not gzipped"))
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
				defer func() {
					mustClose(t, gr)
				}()
				raw := mustReadAll(t, gr)
				if err := json.Unmarshal(raw, &got.metric); err != nil {
					t.Fatalf("bad json: %v; body=%q", err, string(raw))
				}

				tt.serverReply(w, r)
			}))
			defer srv.Close()

			c, err := New(srv.URL, &http.Client{Timeout: 2 * time.Second}, "")
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
			if got.path != updatePath {
				t.Fatalf("path=%q want %s", got.path, updatePath)
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
		if r.URL.Path != updatePath {
			t.Errorf("path=%q want %s", r.URL.Path, updatePath)
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
		defer func() {
			mustClose(t, gr)
		}()
		raw := mustReadAll(t, gr)
		if err := json.Unmarshal(raw, &captured); err != nil {
			t.Fatalf("bad json: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New(srv.URL, nil, "")
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
				mustWrite(t, w, []byte("ok"))
			},
		},
		{
			name: "gzip_200_ok",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				zw := gzip.NewWriter(w)
				mustWrite(t, zw, []byte("ok"))
				mustClose(t, zw)
			},
		},
		{
			name: "gzip_header_but_plain_body_should_error",
			serverReply: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Encoding", "gzip")
				w.WriteHeader(http.StatusOK)
				mustWrite(t, w, []byte("not gzipped"))
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
				defer func() {
					mustClose(t, gr)
				}()
				raw := mustReadAll(t, gr)
				if err := json.Unmarshal(raw, &got.metrics); err != nil {
					t.Fatalf("bad json: %v; body=%q", err, string(raw))
				}

				tt.serverReply(w, r)
			}))
			defer srv.Close()

			c, err := New(srv.URL, &http.Client{Timeout: 2 * time.Second}, "")
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
			if got.path != updatesPath {
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
	c, err := New("http://example", hc, "")
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

type scriptedRT struct {
	mu    sync.Mutex
	calls int
	steps []func(*http.Request) (*http.Response, error)
}

func (s *scriptedRT) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.calls
	if idx >= len(s.steps) {
		idx = len(s.steps) - 1
	}
	s.calls++
	return s.steps[idx](r)
}

func (s *scriptedRT) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func mkResp(code int, body string, hdr http.Header) *http.Response {
	if hdr == nil {
		hdr = make(http.Header)
	}
	return &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Header:     hdr,
		Body:       io.NopCloser(strings.NewReader(body)),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func Test_isRetryableHTTP(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"httpStatus_502", &httpStatusError{code: 502, msg: "bad gateway"}, true},
		{"httpStatus_503", &httpStatusError{code: 503, msg: "unavailable"}, true},
		{"httpStatus_504", &httpStatusError{code: 504, msg: "timeout"}, true},
		{"httpStatus_429", &httpStatusError{code: 429, msg: "ratelimit"}, true},
		{"httpStatus_400", &httpStatusError{code: 400, msg: "bad"}, false},
		{"netOpError", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, true},
		{"urlErrorTimeout", &url.Error{Op: "Get", URL: "http://x", Err: timeoutErr{}}, true},
		{"connRefused", syscall.ECONNREFUSED, true},
		{"connReset", syscall.ECONNRESET, true},
		{"brokenPipe", syscall.EPIPE, true},
		{"permanentGeneric", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableHTTP(tt.err); got != tt.want {
				t.Fatalf("isRetryableHTTP(%T)=%v want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestSendOne_RetryOnNetworkErrors(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	rt := &scriptedRT{
		steps: []func(*http.Request) (*http.Response, error){
			func(*http.Request) (*http.Response, error) {
				return nil, &net.OpError{Op: "dial", Err: syscall.ECONNRESET}
			},
			func(*http.Request) (*http.Response, error) {
				return nil, &url.Error{Op: "Post", URL: "http://x", Err: timeoutErr{}}
			},
			func(*http.Request) (*http.Response, error) {
				return mkResp(http.StatusOK, "ok", nil), nil
			},
		},
	}
	hc := &http.Client{Transport: rt, Timeout: 2 * time.Second}
	c, err := New("http://example", hc, "")
	if err != nil {
		t.Fatal(err)
	}

	val := 42.0
	if err := c.SendOne(context.Background(), domain.Metrics{ID: "Alloc", MType: "gauge", Value: &val}); err != nil {
		t.Fatalf("SendOne error: %v", err)
	}
	if got := rt.Calls(); got != 3 {
		t.Fatalf("RoundTrip calls=%d want 3", got)
	}
}

func TestSendOne_RetryExhausted(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	rt := &scriptedRT{
		steps: []func(*http.Request) (*http.Response, error){
			func(*http.Request) (*http.Response, error) {
				return nil, &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
			},
		},
	}
	hc := &http.Client{Transport: rt, Timeout: 2 * time.Second}
	c, _ := New("http://example", hc, "")

	val := 1.0
	err := c.SendOne(context.Background(), domain.Metrics{ID: "Alloc", MType: "gauge", Value: &val})
	if err == nil || !strings.Contains(err.Error(), "http do:") {
		t.Fatalf("want http do error, got: %v", err)
	}
	if got := rt.Calls(); got != 4 {
		t.Fatalf("RoundTrip calls=%d want 4 (1 initial + 3 retries)", got)
	}
}

func TestSendOne_NoRetry(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	rt := &scriptedRT{
		steps: []func(*http.Request) (*http.Response, error){
			func(*http.Request) (*http.Response, error) { return nil, errors.New("perm") },
		},
	}
	hc := &http.Client{Transport: rt}
	c, _ := New("http://example", hc, "")

	val := 7.0
	err := c.SendOne(context.Background(), domain.Metrics{ID: "Alloc", MType: "gauge", Value: &val})
	if err == nil || !strings.Contains(err.Error(), "http do:") {
		t.Fatalf("want http do error, got: %v", err)
	}
	if got := rt.Calls(); got != 1 {
		t.Fatalf("RoundTrip calls=%d want 1 (no retry)", got)
	}
}

func TestSendOne_NoRetryOn400(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	rt := &scriptedRT{
		steps: []func(*http.Request) (*http.Response, error){
			func(*http.Request) (*http.Response, error) {
				return mkResp(http.StatusBadRequest, "bad", nil), nil
			},
		},
	}
	hc := &http.Client{Transport: rt}
	c, _ := New("http://example", hc, "")

	val := 3.14
	err := c.SendOne(context.Background(), domain.Metrics{ID: "Alloc", MType: "gauge", Value: &val})
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("want 400 error, got: %v", err)
	}
	if got := rt.Calls(); got != 1 {
		t.Fatalf("RoundTrip calls=%d want 1 (status errors are not retried inside op)", got)
	}
}

func TestSendBatch_Retry(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	rt := &scriptedRT{
		steps: []func(*http.Request) (*http.Response, error){
			func(*http.Request) (*http.Response, error) { return nil, &net.OpError{Op: "write", Err: syscall.EPIPE} },
			func(*http.Request) (*http.Response, error) { return mkResp(http.StatusOK, "ok", nil), nil },
		},
	}
	hc := &http.Client{Transport: rt}
	c, _ := New("http://example", hc, "")

	val := 1.23
	delta := int64(7)
	err := c.SendBatch(context.Background(), []domain.Metrics{
		{ID: "Alloc", MType: "gauge", Value: &val},
		{ID: "PollCount", MType: "counter", Delta: &delta},
	})
	if err != nil {
		t.Fatalf("SendBatch error: %v", err)
	}
	if got := rt.Calls(); got != 2 {
		t.Fatalf("RoundTrip calls=%d want 2", got)
	}
}

func TestSendOne_ContextCancel(t *testing.T) {
	orig := misc.DefaultBackoff
	misc.DefaultBackoff = []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 50 * time.Millisecond}
	defer func() { misc.DefaultBackoff = orig }()

	rt := &scriptedRT{
		steps: []func(*http.Request) (*http.Response, error){
			func(*http.Request) (*http.Response, error) {
				return nil, &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
			},
		},
	}
	hc := &http.Client{Transport: rt}
	c, _ := New("http://example", hc, "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	val := 10.0
	err := c.SendOne(ctx, domain.Metrics{ID: "Alloc", MType: "gauge", Value: &val})
	if err == nil || (!strings.Contains(err.Error(), "http do:") && !errors.Is(err, context.DeadlineExceeded)) {
		t.Fatalf("want context-related error, got: %v", err)
	}
	if calls := rt.Calls(); calls < 1 || calls > 2 {
		t.Fatalf("RoundTrip calls=%d want 1..2 (cancel during backoff)", calls)
	}
}

func TestSendOne_ServerGzipResponse(t *testing.T) {
	rt := &scriptedRT{
		steps: []func(*http.Request) (*http.Response, error){
			func(*http.Request) (*http.Response, error) {
				h := make(http.Header)
				h.Set("Content-Encoding", "gzip")
				var b strings.Builder
				zw := gzip.NewWriter(&nopWriteCloser{&b})
				mustWrite(t, zw, []byte("ok"))
				mustClose(t, zw)
				return mkResp(http.StatusOK, b.String(), h), nil
			},
		},
	}
	hc := &http.Client{Transport: rt}
	c, _ := New("http://example", hc, "")

	val := 1.0
	if err := c.SendOne(context.Background(), domain.Metrics{ID: "Alloc", MType: "gauge", Value: &val}); err != nil {
		t.Fatalf("SendOne error: %v", err)
	}
}

func TestSendOne_NoHashHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s want POST", r.Method)
		}
		if r.URL.Path != updatePath {
			t.Errorf("path=%q want /update", r.URL.Path)
		}

		h := r.Header.Get("HashSHA256")
		if h != "" {
			t.Fatalf("expected no HashSHA256 header, got %q", h)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New(srv.URL, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	val := 3.14
	if err := c.SendOne(context.Background(), domain.Metrics{ID: "Alloc", MType: "gauge", Value: &val}); err != nil {
		t.Fatalf("SendOne error: %v", err)
	}
}

func TestSendOne_HashHeader_Present(t *testing.T) {
	key := "secret-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s want POST", r.Method)
		}
		if r.URL.Path != updatePath {
			t.Errorf("path=%q want /update", r.URL.Path)
		}

		h := r.Header.Get("HashSHA256")
		if h == "" {
			t.Fatal("expected HashSHA256 header to be present")
		}

		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("request not gzipped: %v", err)
		}
		defer func() {
			mustClose(t, gr)
		}()
		raw := mustReadAll(t, gr)

		expected := misc.SumSHA256(raw, key)
		if h != expected {
			t.Fatalf("HashSHA256=%q want %q", h, expected)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New(srv.URL, nil, key)
	if err != nil {
		t.Fatal(err)
	}

	val := 2.71
	if err := c.SendOne(context.Background(), domain.Metrics{ID: "Alloc", MType: "gauge", Value: &val}); err != nil {
		t.Fatalf("SendOne error: %v", err)
	}
}

func TestSendBatch_HashHeader_Present(t *testing.T) {
	key := "batch-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s want POST", r.Method)
		}
		if r.URL.Path != updatesPath {
			t.Errorf("path=%q want /updates", r.URL.Path)
		}

		h := r.Header.Get("HashSHA256")
		if h == "" {
			t.Fatal("expected HashSHA256 header to be present")
		}

		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("request not gzipped: %v", err)
		}
		defer func() {
			mustClose(t, gr)
		}()
		raw := mustReadAll(t, gr)

		expected := misc.SumSHA256(raw, key)
		if h != expected {
			t.Fatalf("HashSHA256=%q want %q", h, expected)
		}

		var ms []domain.Metrics
		if err := json.Unmarshal(raw, &ms); err != nil {
			t.Fatalf("bad json: %v", err)
		}
		if len(ms) != 2 {
			t.Fatalf("want 2 metrics, got %d", len(ms))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
}

type nopWriteCloser struct{ *strings.Builder }

func (n *nopWriteCloser) Write(p []byte) (int, error) { return n.Builder.Write(p) }
func (*nopWriteCloser) Close() error                  { return nil }

func TestGzipBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "valid input",
			input:   []byte("test data"),
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   []byte{},
			wantErr: false,
		},
		{
			name:    "large input",
			input:   bytes.Repeat([]byte("x"), 10000),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := gzipBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("gzipBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && payload == nil {
				t.Error("expected non-nil payload")
				return
			}

			if err == nil {
				compressed := payload.Bytes()
				reader, err := gzip.NewReader(bytes.NewReader(compressed))
				if err != nil {
					t.Errorf("failed to create gzip reader: %v", err)
					return
				}
				defer func() {
					_ = reader.Close()
				}()

				decompressed := new(bytes.Buffer)
				if _, err := decompressed.ReadFrom(reader); err != nil {
					t.Errorf("failed to decompress: %v", err)
					return
				}

				if !bytes.Equal(decompressed.Bytes(), tt.input) {
					t.Errorf("decompressed data does not match original input")
				}

				payload.Release()
			}
		})
	}
}

func TestCompressedPayload(t *testing.T) {
	t.Run("bytes returns nil for nil payload", func(t *testing.T) {
		var p *compressedPayload
		if p.Bytes() != nil {
			t.Error("expected nil for nil payload")
		}
	})

	t.Run("release handles nil payload gracefully", func(t *testing.T) {
		var p *compressedPayload
		p.Release() // Should not panic
	})

	t.Run("bytes returns data after gzip", func(t *testing.T) {
		payload, err := gzipBytes([]byte("test"))
		if err != nil {
			t.Fatalf("gzipBytes failed: %v", err)
		}
		defer payload.Release()

		data := payload.Bytes()
		if data == nil {
			t.Error("expected non-nil data")
		}

		if len(data) == 0 {
			t.Error("expected non-empty data")
		}
	})
}
