package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/store"
)

func Test_run_RegistersHandlers(t *testing.T) {
	var handler http.Handler
	fakeListenAndServe := func(addr string, h http.Handler) error {
		handler = h
		return nil
	}

	if err := run(fakeListenAndServe); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
	if handler == nil {
		t.Fatal("handler was not set by runServer")
	}

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/1.23", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /update expected 200, got %d", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "ok" {
		t.Fatalf("POST /update expected body 'ok', got %q", body)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/value/gauge/test", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /value expected 200, got %d", rec2.Code)
	}
	if got := strings.TrimSpace(rec2.Body.String()); got != "1.23" {
		t.Fatalf("GET /value body = %q, want %q", got, "1.23")
	}

	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("GET / expected 200, got %d", rec3.Code)
	}
	if ct := rec3.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET / content-type = %q, want text/html", ct)
	}
}

func Test_normalizeURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ":8080"},
		{"9090", ":9090"},
		{":7070", ":7070"},
		{"http://localhost:1234", "localhost:1234"},
		{"https://1.2.3.4:7777", "1.2.3.4:7777"},
		{"weird:host", "weird:host"},
	}
	for _, tc := range tests {
		if got := normalizeURL(tc.in); got != tc.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func Test_run_WithRestoreAndSyncSave(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "metrics.json")

	st := store.NewMemStorage()
	_ = st.UpdateGauge("Alloc", 42.5)
	if err := store.SaveToFile(st, file); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	t.Setenv("RESTORE", "true")
	t.Setenv("FILE_STORAGE_PATH", file)
	t.Setenv("STORE_INTERVAL", "0s")

	var handler http.Handler
	fakeListen := func(addr string, h http.Handler) error {
		handler = h
		return nil
	}

	if err := run(fakeListen); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	req := httptest.NewRequest("GET", "/value/gauge/Alloc", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("restore GET code=%d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "42.5" {
		t.Fatalf("restore value=%q, want 42.5", got)
	}

	req2 := httptest.NewRequest("POST", "/update/gauge/Alloc/99.9", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("update code=%d", rec2.Code)
	}

	st2 := store.NewMemStorage()
	if err := store.LoadFromFile(st2, file); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if v, ok := st2.GetGauge("Alloc"); !ok || v != 99.9 {
		t.Fatalf("after sync save got=%v ok=%v, want 99.9", v, ok)
	}
}

func Test_run_PeriodicSave(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "metrics.json")

	t.Setenv("STORE_INTERVAL", "1s")
	t.Setenv("FILE_STORAGE_PATH", file)

	var handler http.Handler
	fakeListen := func(addr string, h http.Handler) error {
		handler = h
		return nil
	}
	if err := run(fakeListen); err != nil {
		t.Fatalf("run error: %v", err)
	}

	req := httptest.NewRequest("POST", "/update/counter/C1/5", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("POST update got %d", rec.Code)
	}

	time.Sleep(1500 * time.Millisecond)

	st2 := store.NewMemStorage()
	if err := store.LoadFromFile(st2, file); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if v, ok := st2.GetCounter("C1"); !ok || v != 5 {
		t.Fatalf("expected counter=5, got %v ok=%v", v, ok)
	}
}
