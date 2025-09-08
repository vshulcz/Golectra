package main

import (
	"bytes"
	"os"
	"testing"
)

func TestApplyServerFlags_SetsEnv(t *testing.T) {
	t.Setenv("ADDRESS", "")
	var out bytes.Buffer
	if err := applyServerFlags([]string{"-a=0.0.0.0:9999"}, &out); err != nil {
		t.Fatalf("applyServerFlags error: %v", err)
	}
	if got := os.Getenv("ADDRESS"); got != "0.0.0.0:9999" {
		t.Fatalf("ADDRESS = %q, want %q", got, "0.0.0.0:9999")
	}
}

func TestApplyServerFlags_UnknownFlag(t *testing.T) {
	var out bytes.Buffer
	if err := applyServerFlags([]string{"-z"}, &out); err == nil {
		t.Fatalf("expected error for unknown flag, got nil")
	}
}
