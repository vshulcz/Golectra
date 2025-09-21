package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vshulcz/Golectra/internal/store/memory"
)

func TestSaveAndLoad(t *testing.T) {
	// Normal save and load
	dir := t.TempDir()
	file := filepath.Join(dir, "metrics.json")

	s1 := memory.NewMemStorage()
	s1.UpdateGauge("Alloc", 123.4)
	s1.UpdateCounter("PollCount", 7)

	if err := SaveToFile(s1, file); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	s2 := memory.NewMemStorage()
	if err := LoadFromFile(s2, file); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	if v, ok := s2.GetGauge("Alloc"); !ok || v != 123.4 {
		t.Errorf("gauge Alloc = %v, ok=%v", v, ok)
	}
	if d, ok := s2.GetCounter("PollCount"); !ok || d != 7 {
		t.Errorf("counter PollCount = %v, ok=%v", d, ok)
	}

	// Empty storage
	dir = t.TempDir()
	file = filepath.Join(dir, "metrics.json")

	s1 = memory.NewMemStorage()
	if err := SaveToFile(s1, file); err != nil {
		t.Fatalf("SaveToFile empty: %v", err)
	}

	s2 = memory.NewMemStorage()
	if err := LoadFromFile(s2, file); err != nil {
		t.Fatalf("LoadFromFile empty: %v", err)
	}

	g, c := s2.Snapshot()
	if len(g) != 0 || len(c) != 0 {
		t.Errorf("expected empty after load, got g=%v c=%v", g, c)
	}
}

func TestLoadFromFile(t *testing.T) {
	// File does not exist
	file := filepath.Join(t.TempDir(), "nope.json")
	s := memory.NewMemStorage()
	if err := LoadFromFile(s, file); err != nil {
		t.Fatalf("LoadFromFile non-existent: %v", err)
	}

	// Bad JSON
	dir := t.TempDir()
	file = filepath.Join(dir, "bad.json")
	if err := os.WriteFile(file, []byte("{not json"), 0644); err != nil {
		t.Fatalf("write bad json: %v", err)
	}
	s = memory.NewMemStorage()
	if err := LoadFromFile(s, file); err == nil {
		t.Fatalf("expected error for bad JSON, got nil")
	}
}

func TestSaveToFile_CreateError(t *testing.T) {
	dir := t.TempDir()
	if err := SaveToFile(memory.NewMemStorage(), dir); err == nil {
		t.Fatalf("expected error when saving to directory path, got nil")
	}
}
