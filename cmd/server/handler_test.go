package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vshulcz/Golectra/internal/store"
	"github.com/vshulcz/Golectra/models"
	"go.uber.org/zap"
)

func TestHandler_UpdateMetric(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		url           string
		wantStatus    int
		wantInGauge   map[string]float64
		wantInCounter map[string]int64
	}{
		{
			name:       "valid gauge",
			method:     http.MethodPost,
			url:        "/update/gauge/testGauge/123.4",
			wantStatus: http.StatusOK,
			wantInGauge: map[string]float64{
				"testGauge": 123.4,
			},
		},
		{
			name:       "valid counter",
			method:     http.MethodPost,
			url:        "/update/counter/testCounter/42",
			wantStatus: http.StatusOK,
			wantInCounter: map[string]int64{
				"testCounter": 42,
			},
		},
		{
			name:       "bad metric type",
			method:     http.MethodPost,
			url:        "/update/unknown/x/1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad gauge value",
			method:     http.MethodPost,
			url:        "/update/gauge/testGauge/not-a-number",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad counter value",
			method:     http.MethodPost,
			url:        "/update/counter/testCounter/not-int",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing name",
			method:     http.MethodPost,
			url:        "/update/gauge//123",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "wrong method",
			method:     http.MethodGet,
			url:        "/update/gauge/x/1",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := store.NewMemStorage()
			h := NewHandler(st)

			logger, _ := zap.NewProduction()
			router := NewRouter(h, logger)

			req := httptest.NewRequest(tt.method, tt.url, nil)
			req.Header.Set("Content-Type", "text/plain")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}

			for k, v := range tt.wantInGauge {
				got, ok := st.GetGauge(k)
				if !ok {
					t.Errorf("expected gauge %q to be set", k)
				}
				if got != v {
					t.Errorf("gauge %q = %v, want %v", k, got, v)
				}
			}
			for k, v := range tt.wantInCounter {
				got, ok := st.GetCounter(k)
				if !ok {
					t.Errorf("expected counter %q to be set", k)
				}
				if got != v {
					t.Errorf("counter %q = %v, want %v", k, got, v)
				}
			}
		})
	}
}

func TestHandler_GetValue_and_Index(t *testing.T) {
	st := store.NewMemStorage()
	h := NewHandler(st)

	logger, _ := zap.NewProduction()
	router := NewRouter(h, logger)

	{
		req := httptest.NewRequest(http.MethodPost, "/update/gauge/g1/10.5", nil)
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("seed gauge failed, code=%d", rec.Code)
		}
	}
	{
		req := httptest.NewRequest(http.MethodPost, "/update/counter/c1/7", nil)
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("seed counter failed, code=%d", rec.Code)
		}
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/value/gauge/g1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET value gauge code=%d", rec.Code)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != "10.5" {
			t.Fatalf("GET value gauge body=%q, want %q", got, "10.5")
		}
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/value/counter/c1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET value counter code=%d", rec.Code)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != "7" {
			t.Fatalf("GET value counter body=%q, want %q", got, "7")
		}
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/value/gauge/unknown", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET value unknown code=%d, want 404", rec.Code)
		}
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET / code=%d", rec.Code)
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("GET / content-type=%q, want text/html", ct)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "g1") || !strings.Contains(body, "10.5") {
			t.Fatalf("GET / html does not contain expected metrics; body=%q", body)
		}
	}
}

func TestHandler_UpdateMetricJSON(t *testing.T) {
	srv := newTestServerJSON(t)
	defer srv.Close()

	tests := []struct {
		name       string
		req        models.Metrics
		wantCode   int
		wantCTJSON bool
		wantField  string
		wantNum    float64
	}{
		{
			name:       "gauge ok",
			req:        func() models.Metrics { v := 123.45; return models.Metrics{ID: "Alloc", MType: "gauge", Value: &v} }(),
			wantCode:   http.StatusOK,
			wantCTJSON: true,
			wantField:  "value",
			wantNum:    123.45,
		},
		{
			name: "counter ok (first delta=3 -> total=3)",
			req: func() models.Metrics {
				d := int64(3)
				return models.Metrics{ID: "PollCount", MType: "counter", Delta: &d}
			}(),
			wantCode:   http.StatusOK,
			wantCTJSON: true,
			wantField:  "delta",
			wantNum:    3,
		},
		{
			name:     "bad: missing value for gauge",
			req:      models.Metrics{ID: "X", MType: "gauge"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "bad: missing delta for counter",
			req:      models.Metrics{ID: "Y", MType: "counter"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "bad: empty id",
			req:      func() models.Metrics { v := 1.0; return models.Metrics{ID: "", MType: "gauge", Value: &v} }(),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "bad: unknown type",
			req:      func() models.Metrics { v := 1.0; return models.Metrics{ID: "Z", MType: "weird", Value: &v} }(),
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, data := doJSON(t, http.MethodPost, srv.URL+"/update", tc.req)
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("status=%d want %d; body=%q", resp.StatusCode, tc.wantCode, string(data))
			}
			if tc.wantCTJSON {
				ct := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(ct, "application/json") {
					t.Fatalf("Content-Type=%q want application/json", ct)
				}
			}
			if resp.StatusCode == http.StatusOK {
				var got models.Metrics
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatalf("unmarshal: %v, body=%q", err, string(data))
				}
				switch tc.wantField {
				case "value":
					if got.Value == nil || *got.Value != tc.wantNum {
						t.Fatalf("value=%v want %v", got.Value, tc.wantNum)
					}
				case "delta":
					if got.Delta == nil || float64(*got.Delta) != tc.wantNum {
						t.Fatalf("delta=%v want %v", got.Delta, tc.wantNum)
					}
				}
			}
		})
	}

	d := int64(4)
	resp, data := doJSON(t, http.MethodPost, srv.URL+"/update",
		models.Metrics{ID: "PollCount", MType: "counter", Delta: &d})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%q", resp.StatusCode, string(data))
	}
	var got models.Metrics
	_ = json.Unmarshal(data, &got)
	if got.Delta == nil || *got.Delta != 7 {
		t.Fatalf("accumulated delta=%v want 7", got.Delta)
	}
}

func TestHandler_GetMetricJSON(t *testing.T) {
	srv := newTestServerJSON(t)
	defer srv.Close()

	{
		v := 111.0
		resp, _ := doJSON(t, http.MethodPost, srv.URL+"/update",
			models.Metrics{ID: "LastGC", MType: "gauge", Value: &v})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("seed gauge status=%d", resp.StatusCode)
		}
	}
	{
		d := int64(5)
		resp, _ := doJSON(t, http.MethodPost, srv.URL+"/update",
			models.Metrics{ID: "PollCount", MType: "counter", Delta: &d})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("seed counter status=%d", resp.StatusCode)
		}
	}

	tests := []struct {
		name     string
		req      models.Metrics
		wantCode int
		check    func(t *testing.T, data []byte)
	}{
		{
			name:     "value gauge ok",
			req:      models.Metrics{ID: "LastGC", MType: "gauge"},
			wantCode: http.StatusOK,
			check: func(t *testing.T, data []byte) {
				var got models.Metrics
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if got.Value == nil || *got.Value != 111 {
					t.Fatalf("value=%v want 111", got.Value)
				}
			},
		},
		{
			name:     "value counter ok",
			req:      models.Metrics{ID: "PollCount", MType: "counter"},
			wantCode: http.StatusOK,
			check: func(t *testing.T, data []byte) {
				var got models.Metrics
				_ = json.Unmarshal(data, &got)
				if got.Delta == nil || *got.Delta != 5 {
					t.Fatalf("delta=%v want 5", got.Delta)
				}
			},
		},
		{
			name:     "unknown id -> 404",
			req:      models.Metrics{ID: "Nope", MType: "gauge"},
			wantCode: http.StatusNotFound,
		},
		{
			name:     "bad type -> 400",
			req:      models.Metrics{ID: "LastGC", MType: "weird"},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, data := doJSON(t, http.MethodPost, srv.URL+"/value", tc.req)
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("status=%d want %d; body=%q", resp.StatusCode, tc.wantCode, string(data))
			}
			if resp.StatusCode == http.StatusOK {
				ct := resp.Header.Get("Content-Type")
				if !strings.HasPrefix(ct, "application/json") {
					t.Fatalf("Content-Type=%q want application/json", ct)
				}
				if tc.check != nil {
					tc.check(t, data)
				}
			}
		})
	}
}

func newTestServerJSON(t *testing.T) *httptest.Server {
	t.Helper()
	st := store.NewMemStorage()
	h := NewHandler(st)
	r := NewRouter(h, zap.NewNop())
	return httptest.NewServer(r)
}

func doJSON(t *testing.T, method, url string, payload any) (*http.Response, []byte) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, data
}

func TestHandler_AfterUpdateCalled(t *testing.T) {
	st := store.NewMemStorage()
	h := NewHandler(st)

	called := false
	h.SetAfterUpdate(func() { called = true })

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/x/1.23", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	r := NewRouter(h, zap.NewNop())
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	if !called {
		t.Fatalf("afterUpdate should have been called")
	}

	called = false
	v := 5.5
	reqJSON, _ := json.Marshal(models.Metrics{ID: "y", MType: "gauge", Value: &v})
	req = httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	if !called {
		t.Fatalf("afterUpdate should have been called for JSON")
	}
}

func TestHandler_UpdateMetricJSON_BadPayloads(t *testing.T) {
	st := store.NewMemStorage()
	h := NewHandler(st)
	r := NewRouter(h, zap.NewNop())

	tests := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"invalid json", "{"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d, want 400", rec.Code)
			}
		})
	}
}

func TestHandler_GetMetricJSON_BadPayloads(t *testing.T) {
	st := store.NewMemStorage()
	h := NewHandler(st)
	r := NewRouter(h, zap.NewNop())

	tests := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"invalid json", "{"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/value", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d, want 400", rec.Code)
			}
		})
	}
}
