package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	filemem "github.com/vshulcz/Golectra/internal/store/file"
	"github.com/vshulcz/Golectra/internal/store/memory"
)

func Test_run_UsesConfig(t *testing.T) {
	withIsolatedPersistence(t)
	t.Setenv("ADDRESS", ":12345")

	var gotAddr string
	var gotHandler http.Handler
	fakeListen := func(addr string, h http.Handler) error {
		gotAddr, gotHandler = addr, h
		return nil
	}
	if err := run(fakeListen); err != nil {
		t.Fatalf("run error: %v", err)
	}
	if gotAddr != ":12345" {
		t.Errorf("listen addr=%q, want :12345", gotAddr)
	}
	if gotHandler == nil {
		t.Fatal("listen handler was nil")
	}
}
func Test_run_RegistersBasicEndpoints(t *testing.T) {
	withIsolatedPersistence(t)

	var handler http.Handler
	fakeListen := func(_ string, h http.Handler) error {
		handler = h
		return nil
	}
	if err := run(fakeListen); err != nil {
		t.Fatalf("run error: %v", err)
	}

	req := httptest.NewRequest("POST", "/update/gauge/CPU/1.5", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST /update status=%d, want 200", rec.Code)
	}

	req2 := httptest.NewRequest("GET", "/value/gauge/CPU", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("GET /value status=%d, want 200", rec2.Code)
	}
	if got := strings.TrimSpace(rec2.Body.String()); got != "1.5" {
		t.Errorf("GET /value body=%q, want 1.5", got)
	}
}

func Test_run_RestoreFromFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "restore.json")

	st := memory.NewMemStorage()
	_ = st.UpdateGauge("X", 3.14)
	if err := filemem.SaveToFile(st, file); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	t.Setenv("RESTORE", "true")
	t.Setenv("FILE_STORAGE_PATH", file)
	t.Setenv("STORE_INTERVAL", "0s")

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"server"}

	var handler http.Handler
	fakeListen := func(_ string, h http.Handler) error {
		handler = h
		return nil
	}
	if err := run(fakeListen); err != nil {
		t.Fatalf("run error: %v", err)
	}

	req := httptest.NewRequest("GET", "/value/gauge/X", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("restore GET code=%d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "3.14" {
		t.Fatalf("restore value=%q, want 3.14", got)
	}
}

func Test_run_SyncSave(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "sync.json")

	t.Setenv("FILE_STORAGE_PATH", file)
	t.Setenv("STORE_INTERVAL", "0s")

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"server"}

	var handler http.Handler
	fakeListen := func(_ string, h http.Handler) error {
		handler = h
		return nil
	}
	if err := run(fakeListen); err != nil {
		t.Fatalf("run error: %v", err)
	}

	req := httptest.NewRequest("POST", "/update/gauge/M1/99.9", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("POST /update got %d", rec.Code)
	}

	st2 := memory.NewMemStorage()
	if err := filemem.LoadFromFile(st2, file); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if v, ok := st2.GetGauge("M1"); !ok || v != 99.9 {
		t.Fatalf("after sync save got=%v ok=%v, want 99.9", v, ok)
	}
}

func Test_run_PeriodicSave(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "periodic.json")

	t.Setenv("STORE_INTERVAL", "1s")
	t.Setenv("FILE_STORAGE_PATH", file)

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"server"}

	var handler http.Handler
	fakeListen := func(_ string, h http.Handler) error {
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
		t.Fatalf("POST /update got %d", rec.Code)
	}

	time.Sleep(1500 * time.Millisecond)

	st2 := memory.NewMemStorage()
	if err := filemem.LoadFromFile(st2, file); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if v, ok := st2.GetCounter("C1"); !ok || v != 5 {
		t.Fatalf("expected counter=5, got %v ok=%v", v, ok)
	}
}

func withIsolatedPersistence(t *testing.T) (tmpFile string) {
	t.Helper()

	dir := t.TempDir()
	tmpFile = filepath.Join(dir, "state.json")

	t.Setenv("FILE_STORAGE_PATH", tmpFile)
	t.Setenv("STORE_INTERVAL", "0s")
	t.Setenv("RESTORE", "false")
	t.Setenv("GIN_MODE", "release")

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"server"}

	return tmpFile
}
