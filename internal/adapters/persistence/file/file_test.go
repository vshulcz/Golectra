package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vshulcz/Golectra/internal/adapters/repository/memory"
)

func TestSave(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "metrics.json")

	s1 := memory.New()
	s1.SetGauge(context.TODO(), "Alloc", 123.4)
	s1.AddCounter(context.TODO(), "PollCount", 7)

	p := New(file)

	s, _ := s1.Snapshot(context.TODO())
	if err := p.Save(context.TODO(), s); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	s2 := memory.New()
	if err := p.Restore(context.TODO(), s2); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	if v, err := s2.GetGauge(context.TODO(), "Alloc"); err != nil || v != 123.4 {
		t.Errorf("gauge Alloc = %v, err=%v", v, err)
	}
	if d, err := s2.GetCounter(context.TODO(), "PollCount"); err != nil || d != 7 {
		t.Errorf("counter PollCount = %v, err=%v", d, err)
	}

	dir = t.TempDir()
	file = filepath.Join(dir, "metrics.json")
	p = New(file)

	s1 = memory.New()
	s, _ = s1.Snapshot(context.TODO())
	if err := p.Save(context.TODO(), s); err != nil {
		t.Fatalf("Save empty: %v", err)
	}

	s2 = memory.New()
	if err := p.Restore(context.TODO(), s2); err != nil {
		t.Fatalf("Restore empty: %v", err)
	}

	s, _ = s2.Snapshot(context.TODO())
	if len(s.Gauges) != 0 || len(s.Counters) != 0 {
		t.Errorf("expected empty after load, got g=%v c=%v", s.Gauges, s.Counters)
	}
}

func TestRestore(t *testing.T) {
	file := filepath.Join(t.TempDir(), "nope.json")
	p := New(file)
	s := memory.New()
	if err := p.Restore(context.TODO(), s); err != nil {
		t.Fatalf("Restore non-existent: %v", err)
	}

	dir := t.TempDir()
	file = filepath.Join(dir, "bad.json")
	if err := os.WriteFile(file, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write bad json: %v", err)
	}
	p = New(file)
	s = memory.New()
	if err := p.Restore(context.TODO(), s); err == nil {
		t.Fatal("expected error for bad JSON, got nil")
	}
}

func TestSave_CreateError(t *testing.T) {
	dir := t.TempDir()
	p := New(dir)
	s, _ := memory.New().Snapshot(context.TODO())
	if err := p.Save(context.TODO(), s); err == nil {
		t.Fatal("expected error when saving to directory path, got nil")
	}
}
