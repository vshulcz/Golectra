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

	err := run(fakeListenAndServe)
	if err != nil {
		t.Fatalf("runServer returned error: %v", err)
	}
	if handler == nil {
		t.Fatal("handler was not set by runServer")
	}

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/1.23", strings.NewReader(""))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("expected body 'ok', got %q", body)
	}
}
