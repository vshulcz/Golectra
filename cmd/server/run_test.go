package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
