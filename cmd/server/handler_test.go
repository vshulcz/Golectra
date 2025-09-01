package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vshulcz/Golectra/internal/store"
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
			router := NewRouter(h)

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
	router := NewRouter(h)

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
