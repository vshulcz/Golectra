package main

import (
	"bytes"
	"os"
	"testing"
)

func TestApplyServerFlags_SetsEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	var out bytes.Buffer
	if err := applyServerFlags([]string{"-a=0.0.0.0:9999"}, &out); err != nil {
		t.Fatalf("applyServerFlags error: %v", err)
	}
	if got := os.Getenv("HTTP_ADDR"); got != "0.0.0.0:9999" {
		t.Fatalf("HTTP_ADDR = %q, want %q", got, "0.0.0.0:9999")
	}
}

func TestApplyServerFlags_UnknownFlag(t *testing.T) {
	var out bytes.Buffer
	if err := applyServerFlags([]string{"-z"}, &out); err == nil {
		t.Fatalf("expected error for unknown flag, got nil")
	}
}
