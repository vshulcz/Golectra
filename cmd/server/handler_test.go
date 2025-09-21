package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/store"
	"go.uber.org/zap"
)

func newServer(t *testing.T, st store.Storage) *httptest.Server {
	t.Helper()
	h := NewHandler(st)
	r := NewRouter(h, zap.NewNop())
	return httptest.NewServer(r)
}

func doReq(t *testing.T, method, url string, body []byte, hdr map[string]string) (*http.Response, []byte) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	data := readMaybeGzip(t, resp)
	return resp, data
}

func readMaybeGzip(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	var r io.Reader = resp.Body
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		zr, err := gzip.NewReader(resp.Body)
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		defer zr.Close()
		r = zr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func TestHTTP_PathAndHTML(t *testing.T) {
	srv := newServer(t, store.NewMemStorage())
	defer srv.Close()

	type tc struct {
		name       string
		method     string
		path       string
		hdr        map[string]string
		bodySubstr string
		wantCode   int
		after      func(*testing.T)
	}

	tests := []tc{
		{
			name:     "update gauge ok",
			method:   http.MethodPost,
			path:     "/update/gauge/testGauge/123.4",
			hdr:      map[string]string{"Content-Type": "text/plain"},
			wantCode: http.StatusOK,
		},
		{
			name:     "update counter ok",
			method:   http.MethodPost,
			path:     "/update/counter/testCounter/42",
			hdr:      map[string]string{"Content-Type": "text/plain"},
			wantCode: http.StatusOK,
		},
		{
			name:     "update bad type",
			method:   http.MethodPost,
			path:     "/update/unknown/x/1",
			hdr:      map[string]string{"Content-Type": "text/plain"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "update gauge bad value",
			method:   http.MethodPost,
			path:     "/update/gauge/testGauge/not-a-number",
			hdr:      map[string]string{"Content-Type": "text/plain"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "update counter bad value",
			method:   http.MethodPost,
			path:     "/update/counter/testCounter/not-int",
			hdr:      map[string]string{"Content-Type": "text/plain"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "update missing name",
			method:   http.MethodPost,
			path:     "/update/gauge//123",
			hdr:      map[string]string{"Content-Type": "text/plain"},
			wantCode: http.StatusNotFound,
		},
		{
			name:     "wrong method for update",
			method:   http.MethodGet,
			path:     "/update/gauge/x/1",
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name:   "seed gauge",
			method: http.MethodPost, path: "/update/gauge/g1/10.5",
			hdr:      map[string]string{"Content-Type": "text/plain"},
			wantCode: http.StatusOK,
		},
		{
			name:   "get gauge ok",
			method: http.MethodGet, path: "/value/gauge/g1",
			wantCode: http.StatusOK, bodySubstr: "10.5",
		},
		{
			name:   "seed counter",
			method: http.MethodPost, path: "/update/counter/c1/7",
			hdr:      map[string]string{"Content-Type": "text/plain"},
			wantCode: http.StatusOK,
		},
		{
			name:   "get counter ok",
			method: http.MethodGet, path: "/value/counter/c1",
			wantCode: http.StatusOK, bodySubstr: "7",
		},
		{
			name:   "get unknown -> 404",
			method: http.MethodGet, path: "/value/gauge/unknown",
			wantCode: http.StatusNotFound,
		},
		{
			name:   "index html (gzipped)",
			method: http.MethodGet, path: "/",
			hdr:      map[string]string{"Accept-Encoding": "gzip"},
			wantCode: http.StatusOK,
			after: func(t *testing.T) {
				resp, body := doReq(t, http.MethodGet, srv.URL+"/", nil, map[string]string{"Accept-Encoding": "gzip"})
				if ce := resp.Header.Get("Content-Encoding"); !strings.Contains(strings.ToLower(ce), "gzip") {
					t.Fatalf("html Content-Encoding=%q want gzip", ce)
				}
				if ct := http.DetectContentType(body); !strings.HasPrefix(ct, "text/html") {
					t.Fatalf("html content-type detect=%q", ct)
				}
				if !strings.Contains(string(body), "g1") || !strings.Contains(string(body), "10.5") {
					t.Fatalf("html body missing seeded metrics")
				}
			},
		},
		{
			name:   "404 should not be gzipped",
			method: http.MethodGet, path: "/unknown",
			hdr:      map[string]string{"Accept-Encoding": "gzip"},
			wantCode: http.StatusNotFound,
			after: func(t *testing.T) {
				resp, _ := doReq(t, http.MethodGet, srv.URL+"/unknown", nil, map[string]string{"Accept-Encoding": "gzip"})
				if ce := resp.Header.Get("Content-Encoding"); ce != "" {
					t.Fatalf("404 must not be gzipped, got %q", ce)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, body := doReq(t, tc.method, srv.URL+tc.path, nil, tc.hdr)
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("status=%d want %d; body=%q", resp.StatusCode, tc.wantCode, string(body))
			}
			if tc.bodySubstr != "" && !strings.Contains(string(body), tc.bodySubstr) {
				t.Fatalf("body %q does not contain %q", string(body), tc.bodySubstr)
			}
			if tc.after != nil {
				tc.after(t)
			}
		})
	}
}

func TestHTTP_JSON(t *testing.T) {
	srv := newServer(t, store.NewMemStorage())
	defer srv.Close()

	type jtc struct {
		name     string
		urlPath  string
		payload  domain.Metrics
		gzipReq  bool
		wantCode int
		wantJSON func(t *testing.T, b []byte)
		wantGzip bool
	}

	val := func(f float64) *float64 { return &f }
	dlt := func(i int64) *int64 { return &i }

	cases := []jtc{
		{"update gauge ok", "/update", domain.Metrics{ID: "Alloc", MType: "gauge", Value: val(123.45)}, false, http.StatusOK,
			func(t *testing.T, b []byte) {
				var got domain.Metrics
				_ = json.Unmarshal(b, &got)
				if got.Value == nil || *got.Value != 123.45 {
					t.Fatalf("got=%+v", got)
				}
			}, false},
		{"update counter ok", "/update", domain.Metrics{ID: "PollCount", MType: "counter", Delta: dlt(3)}, false, http.StatusOK,
			func(t *testing.T, b []byte) {
				var got domain.Metrics
				_ = json.Unmarshal(b, &got)
				if got.Delta == nil || *got.Delta != 3 {
					t.Fatalf("got=%+v", got)
				}
			}, false},

		{"update bad: missing value for gauge", "/update", domain.Metrics{ID: "X", MType: "gauge"}, false, http.StatusBadRequest, nil, false},
		{"update bad: missing delta for counter", "/update", domain.Metrics{ID: "Y", MType: "counter"}, false, http.StatusBadRequest, nil, false},
		{"update bad: empty id", "/update", domain.Metrics{ID: "", MType: "gauge", Value: val(1)}, false, http.StatusBadRequest, nil, false},
		{"update bad: unknown type", "/update", domain.Metrics{ID: "Z", MType: "weird", Value: val(1)}, false, http.StatusBadRequest, nil, false},

		{"update accepts gzip body", "/update", domain.Metrics{ID: "LastGC", MType: "gauge", Value: val(777)}, true, http.StatusOK,
			func(t *testing.T, b []byte) {
				var got domain.Metrics
				_ = json.Unmarshal(b, &got)
				if got.Value == nil || *got.Value != 777 {
					t.Fatalf("got=%+v", got)
				}
			}, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.payload)
			h := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
			if tc.gzipReq {
				b = gzipBytes(t, b)
				h["Content-Encoding"] = "gzip"
			}
			resp, body := doReq(t, http.MethodPost, srv.URL+tc.urlPath, b, h)
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("status=%d want %d; body=%q", resp.StatusCode, tc.wantCode, string(body))
			}
			if ct := resp.Header.Get("Content-Type"); resp.StatusCode == http.StatusOK && !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("Content-Type=%q want application/json", ct)
			}
			if tc.wantJSON != nil {
				tc.wantJSON(t, body)
			}
		})
	}
	{
		b, _ := json.Marshal(domain.Metrics{ID: "PollCount", MType: "counter", Delta: dlt(4)})
		resp, body := doReq(t, http.MethodPost, srv.URL+"/update", b, map[string]string{"Content-Type": "application/json"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%q", resp.StatusCode, string(body))
		}
		var got domain.Metrics
		_ = json.Unmarshal(body, &got)
		if got.Delta == nil || *got.Delta != 7 {
			t.Fatalf("accumulated delta=%v want 7", got.Delta)
		}
	}
	{
		q, _ := json.Marshal(domain.Metrics{ID: "Alloc", MType: "gauge"})
		resp, body := doReq(t, http.MethodPost, srv.URL+"/value", q,
			map[string]string{"Content-Type": "application/json", "Accept-Encoding": "gzip"})
		if ce := resp.Header.Get("Content-Encoding"); !strings.Contains(strings.ToLower(ce), "gzip") {
			t.Fatalf("Content-Encoding=%q want gzip", ce)
		}
		var got domain.Metrics
		_ = json.Unmarshal(body, &got)
		if got.Value == nil || *got.Value != 123.45 {
			t.Fatalf("got=%+v", got)
		}
	}
	resp, _ := doReq(t, http.MethodPost, srv.URL+"/value", mustJSON(domain.Metrics{ID: "Nope", MType: "gauge"}),
		map[string]string{"Content-Type": "application/json"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
	resp, _ = doReq(t, http.MethodPost, srv.URL+"/value", mustJSON(domain.Metrics{ID: "Alloc", MType: "weird"}),
		map[string]string{"Content-Type": "application/json"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}

	resp, _ = doReq(t, http.MethodPost, srv.URL+"/update", nil, map[string]string{"Content-Type": "application/json"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty body: want 400, got %d", resp.StatusCode)
	}
	resp, _ = doReq(t, http.MethodPost, srv.URL+"/value", []byte("{"), map[string]string{"Content-Type": "application/json"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid json: want 400, got %d", resp.StatusCode)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

type pingOKStorage struct{ store.Storage }

func (p *pingOKStorage) Ping() error { return nil }

func TestHTTP_Ping(t *testing.T) {
	t.Run("mem storage -> 500 (db not configured)", func(t *testing.T) {
		srv := newServer(t, store.NewMemStorage())
		defer srv.Close()
		resp, _ := doReq(t, http.MethodGet, srv.URL+"/ping", nil, nil)
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status=%d want 500", resp.StatusCode)
		}
	})

	t.Run("ok when storage.Ping()==nil", func(t *testing.T) {
		srv := newServer(t, &pingOKStorage{Storage: store.NewMemStorage()})
		defer srv.Close()
		resp, _ := doReq(t, http.MethodGet, srv.URL+"/ping", nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d want 200", resp.StatusCode)
		}
	})
}
