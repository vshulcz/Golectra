package ginserver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
	"github.com/vshulcz/Golectra/internal/services/metrics"
	"go.uber.org/zap"

	"github.com/vshulcz/Golectra/internal/adapters/http/ginserver/middlewares"
	memrepo "github.com/vshulcz/Golectra/internal/adapters/repository/memory"
)

func newServer(t *testing.T, repo ports.MetricsRepo, onChanged ...func(context.Context, domain.Snapshot)) *httptest.Server {
	t.Helper()

	var hook func(context.Context, domain.Snapshot)
	if len(onChanged) > 0 {
		hook = onChanged[0]
	}

	svc := metrics.New(repo, hook)
	h := NewHandler(svc)

	r := NewRouter(
		h,
		zap.NewNop(),
		middlewares.ZapLogger(zap.NewNop()),
		middlewares.GzipRequest(),
		middlewares.GzipResponse(),
	)
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
	srv := newServer(t, memrepo.New())
	defer srv.Close()

	type tc struct {
		hdr        map[string]string
		after      func(*testing.T)
		name       string
		method     string
		path       string
		bodySubstr string
		wantCode   int
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
					t.Fatal("html body missing seeded metrics")
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
	srv := newServer(t, memrepo.New())
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
		{
			"update gauge ok", "/update",
			domain.Metrics{ID: "Alloc", MType: "gauge", Value: val(123.45)},
			false, http.StatusOK,
			func(t *testing.T, b []byte) {
				var got domain.Metrics
				json.Unmarshal(b, &got)
				if got.Value == nil || *got.Value != 123.45 {
					t.Fatalf("got=%+v", got)
				}
			}, false,
		},
		{
			"update counter ok", "/update",
			domain.Metrics{ID: "PollCount", MType: "counter", Delta: dlt(3)},
			false, http.StatusOK,
			func(t *testing.T, b []byte) {
				var got domain.Metrics
				json.Unmarshal(b, &got)
				if got.Delta == nil || *got.Delta != 3 {
					t.Fatalf("got=%+v", got)
				}
			}, false,
		},

		{"update bad: missing value for gauge", "/update", domain.Metrics{ID: "X", MType: "gauge"}, false, http.StatusBadRequest, nil, false},
		{"update bad: missing delta for counter", "/update", domain.Metrics{ID: "Y", MType: "counter"}, false, http.StatusBadRequest, nil, false},
		{"update bad: empty id", "/update", domain.Metrics{ID: "", MType: "gauge", Value: val(1)}, false, http.StatusBadRequest, nil, false},
		{"update bad: unknown type", "/update", domain.Metrics{ID: "Z", MType: "weird", Value: val(1)}, false, http.StatusBadRequest, nil, false},

		{
			"update accepts gzip body", "/update",
			domain.Metrics{ID: "LastGC", MType: "gauge", Value: val(777)},
			true, http.StatusOK,
			func(t *testing.T, b []byte) {
				var got domain.Metrics
				json.Unmarshal(b, &got)
				if got.Value == nil || *got.Value != 777 {
					t.Fatalf("got=%+v", got)
				}
			}, false,
		},
	}

	for _, tc := range cases {
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
		json.Unmarshal(body, &got)
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
		json.Unmarshal(body, &got)
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

type pingOKRepo struct{ *memrepo.Repo }

func (*pingOKRepo) Ping(context.Context) error { return nil }

func TestHTTP_Ping(t *testing.T) {
	t.Run("mem storage -> 500 (db not configured)", func(t *testing.T) {
		srv := newServer(t, memrepo.New())
		defer srv.Close()
		resp, _ := doReq(t, http.MethodGet, srv.URL+"/ping", nil, nil)
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status=%d want 500", resp.StatusCode)
		}
	})

	t.Run("ok when storage.Ping()==nil", func(t *testing.T) {
		srv := newServer(t, &pingOKRepo{Repo: memrepo.New()})
		defer srv.Close()
		resp, _ := doReq(t, http.MethodGet, srv.URL+"/ping", nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d want 200", resp.StatusCode)
		}
	})
}

func TestHTTP_UpdatesBatch(t *testing.T) {
	type updResp struct {
		Updated int `json:"updated"`
	}

	t.Run("success_plain_and_gzip", func(t *testing.T) {
		srv := newServer(t, memrepo.New())
		defer srv.Close()

		val := func(f float64) *float64 { return &f }
		dlt := func(i int64) *int64 { return &i }

		payload := []domain.Metrics{
			{ID: "g1", MType: "gauge", Value: val(3.14)},
			{ID: "c1", MType: "counter", Delta: dlt(5)},
		}
		body, _ := json.Marshal(payload)

		{
			resp, raw := doReq(t, http.MethodPost, srv.URL+"/updates", body,
				map[string]string{"Content-Type": "application/json", "Accept": "application/json"})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status=%d body=%q", resp.StatusCode, string(raw))
			}
			var ur updResp
			json.Unmarshal(raw, &ur)
			if ur.Updated != 2 {
				t.Fatalf("updated=%d want 2", ur.Updated)
			}
		}

		{
			resp, raw := doReq(t, http.MethodPost, srv.URL+"/value", mustJSON(domain.Metrics{ID: "g1", MType: "gauge"}),
				map[string]string{"Content-Type": "application/json"})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("get gauge: status=%d body=%q", resp.StatusCode, string(raw))
			}
			var got domain.Metrics
			json.Unmarshal(raw, &got)
			if got.Value == nil || *got.Value != 3.14 {
				t.Fatalf("g1=%+v want 3.14", got)
			}
		}

		{
			gzBody := gzipBytes(t, body)
			resp, raw := doReq(t, http.MethodPost, srv.URL+"/updates", gzBody,
				map[string]string{
					"Content-Type":     "application/json",
					"Content-Encoding": "gzip",
					"Accept":           "application/json",
					"Accept-Encoding":  "gzip",
				})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status=%d body=%q", resp.StatusCode, string(raw))
			}
			if ce := resp.Header.Get("Content-Encoding"); !strings.Contains(strings.ToLower(ce), "gzip") {
				t.Fatalf("response must be gzipped; got %q", ce)
			}
			var ur updResp
			json.Unmarshal(raw, &ur)
			if ur.Updated != 2 {
				t.Fatalf("updated=%d want 2", ur.Updated)
			}
		}
	})

	t.Run("trailing_slash_route", func(t *testing.T) {
		srv := newServer(t, memrepo.New())
		defer srv.Close()

		val := func(f float64) *float64 { return &f }
		body, _ := json.Marshal([]domain.Metrics{
			{ID: "gts", MType: "gauge", Value: val(1)},
		})
		resp, raw := doReq(t, http.MethodPost, srv.URL+"/updates/", body,
			map[string]string{"Content-Type": "application/json", "Accept": "application/json"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%q", resp.StatusCode, string(raw))
		}
	})

	t.Run("bad_json_and_empty_and_all_invalid", func(t *testing.T) {
		srv := newServer(t, memrepo.New())
		defer srv.Close()

		{
			resp, _ := doReq(t, http.MethodPost, srv.URL+"/updates", []byte("{"),
				map[string]string{"Content-Type": "application/json"})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("bad json: want 400, got %d", resp.StatusCode)
			}
		}
		{
			resp, _ := doReq(t, http.MethodPost, srv.URL+"/updates", []byte("[]"),
				map[string]string{"Content-Type": "application/json"})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("empty array: want 400, got %d", resp.StatusCode)
			}
		}
		{
			body, _ := json.Marshal([]domain.Metrics{
				{ID: "", MType: "gauge"},
				{ID: "x", MType: "gauge"},
				{ID: "y", MType: "counter"},
				{ID: "z", MType: "unknown", Value: nil},
			})
			resp, raw := doReq(t, http.MethodPost, srv.URL+"/updates", body,
				map[string]string{"Content-Type": "application/json"})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("all invalid: want 400, got %d (body=%q)", resp.StatusCode, string(raw))
			}
		}
	})

	t.Run("partially_invalid_filtered_but_updates_happen", func(t *testing.T) {
		srv := newServer(t, memrepo.New())
		defer srv.Close()

		val := func(f float64) *float64 { return &f }
		dlt := func(i int64) *int64 { return &i }
		body, _ := json.Marshal([]domain.Metrics{
			{ID: "ok1", MType: "gauge", Value: val(10)},
			{ID: "bad1", MType: "gauge"},
			{ID: "ok2", MType: "counter", Delta: dlt(5)},
			{ID: "", MType: "counter", Delta: dlt(1)},
			{ID: "bad2", MType: "weird", Value: val(1)},
		})

		resp, raw := doReq(t, http.MethodPost, srv.URL+"/updates", body,
			map[string]string{"Content-Type": "application/json", "Accept": "application/json"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%q", resp.StatusCode, string(raw))
		}
		var ur struct{ Updated int }
		json.Unmarshal(raw, &ur)
		if ur.Updated != 2 {
			t.Fatalf("updated=%d want 2", ur.Updated)
		}

		{
			resp, raw := doReq(t, http.MethodPost, srv.URL+"/value", mustJSON(domain.Metrics{ID: "ok1", MType: "gauge"}),
				map[string]string{"Content-Type": "application/json"})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("get ok1: %d", resp.StatusCode)
			}
			var got domain.Metrics
			json.Unmarshal(raw, &got)
			if got.Value == nil || *got.Value != 10 {
				t.Fatalf("ok1=%+v", got)
			}
		}
		{
			resp, raw := doReq(t, http.MethodPost, srv.URL+"/value", mustJSON(domain.Metrics{ID: "ok2", MType: "counter"}),
				map[string]string{"Content-Type": "application/json"})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("get ok2: %d", resp.StatusCode)
			}
			var got domain.Metrics
			json.Unmarshal(raw, &got)
			if got.Delta == nil || *got.Delta != 5 {
				t.Fatalf("ok2=%+v", got)
			}
		}
	})

	t.Run("updateMany_error_returns_500_not_gzipped", func(t *testing.T) {
		srv := newServer(t, &errUpdateManyRepo{})
		defer srv.Close()

		val := func(f float64) *float64 { return &f }
		body, _ := json.Marshal([]domain.Metrics{{ID: "g1", MType: "gauge", Value: val(1)}})

		resp, raw := doReq(t, http.MethodPost, srv.URL+"/updates", body,
			map[string]string{
				"Content-Type":    "application/json",
				"Accept-Encoding": "gzip",
			})
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", resp.StatusCode, string(raw))
		}
		if ce := resp.Header.Get("Content-Encoding"); ce != "" {
			t.Fatalf("500 must not be gzipped, got %q", ce)
		}
	})

	t.Run("onChanged_is_called_once_on_success", func(t *testing.T) {
		repo := memrepo.New()

		called := make(chan struct{}, 1)
		hook := func(_ context.Context, _ domain.Snapshot) {
			select {
			case called <- struct{}{}:
			default:
			}
		}
		srv := newServer(t, repo, hook)
		defer srv.Close()

		val := func(f float64) *float64 { return &f }
		body, _ := json.Marshal([]domain.Metrics{{ID: "g1", MType: "gauge", Value: val(1)}})

		resp, raw := doReq(t, http.MethodPost, srv.URL+"/updates", body,
			map[string]string{"Content-Type": "application/json"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%q", resp.StatusCode, string(raw))
		}
		select {
		case <-called:
		default:
			t.Fatal("onChanged was not called")
		}
	})
}

type errUpdateManyRepo struct{}

func (*errUpdateManyRepo) GetGauge(context.Context, string) (float64, error) {
	return 0, domain.ErrNotFound
}

func (*errUpdateManyRepo) GetCounter(context.Context, string) (int64, error) {
	return 0, domain.ErrNotFound
}
func (*errUpdateManyRepo) SetGauge(context.Context, string, float64) error { return nil }
func (*errUpdateManyRepo) AddCounter(context.Context, string, int64) error { return nil }
func (*errUpdateManyRepo) UpdateMany(context.Context, []domain.Metrics) error {
	return errors.New("boom")
}

func (*errUpdateManyRepo) Snapshot(context.Context) (domain.Snapshot, error) {
	return domain.Snapshot{Gauges: map[string]float64{}, Counters: map[string]int64{}}, nil
}
func (*errUpdateManyRepo) Ping(context.Context) error { return errors.New("db not configured") }

func TestSnapshotJSON_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := memrepo.New()
	if err := repo.SetGauge(context.TODO(), "CPUutilization1", 12.34); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetGauge(context.TODO(), "HeapAlloc", 111); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddCounter(context.TODO(), "PollCount", 7); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(metrics.New(repo, nil))
	r := gin.New()
	r.GET("/api/v1/snapshot", h.SnapshotJSON)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/snapshot", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, body=%s", w.Code, w.Body.String())
	}

	var got struct {
		Gauges   map[string]float64 `json:"gauges"`
		Counters map[string]int64   `json:"counters"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Gauges["CPUutilization1"] != 12.34 {
		t.Fatalf("gauges.CPUutilization1=%v", got.Gauges["CPUutilization1"])
	}
	if got.Counters["PollCount"] != 7 {
		t.Fatalf("counters.PollCount=%v", got.Counters["PollCount"])
	}
}
