package util

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintBuildInfo(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	os.Stdout = w
	PrintBuildInfo("v1", "2026-01-01", "abc123")
	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Build version: v1", "Build date: 2026-01-01", "Build commit: abc123"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
}
