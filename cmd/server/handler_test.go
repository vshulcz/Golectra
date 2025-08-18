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

			req := httptest.NewRequest(tt.method, tt.url, strings.NewReader(""))
			rec := httptest.NewRecorder()

			h.UpdateMetric(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
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
