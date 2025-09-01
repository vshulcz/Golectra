package main

import (
	"bytes"
	"os"
	"testing"
)

func TestApplyAgentFlags_SetsEnv_All(t *testing.T) {
	t.Setenv("SERVER_URL", "")
	t.Setenv("REPORT_INTERVAL", "")
	t.Setenv("POLL_INTERVAL", "")

	var out bytes.Buffer
	err := applyAgentFlags([]string{"-a=localhost:9090", "-r=5", "-p=1"}, &out)
	if err != nil {
		t.Fatalf("applyAgentFlags error: %v", err)
	}

	if got := os.Getenv("SERVER_URL"); got != "http://localhost:9090" {
		t.Fatalf("SERVER_URL = %q, want %q", got, "http://localhost:9090")
	}
	if got := os.Getenv("REPORT_INTERVAL"); got != "5s" {
		t.Fatalf("REPORT_INTERVAL = %q, want %q", got, "5s")
	}
	if got := os.Getenv("POLL_INTERVAL"); got != "1s" {
		t.Fatalf("POLL_INTERVAL = %q, want %q", got, "1s")
	}
}

func TestApplyAgentFlags_NormalizesScheme(t *testing.T) {
	t.Setenv("SERVER_URL", "")
	var out bytes.Buffer

	if err := applyAgentFlags([]string{"-a=https://example:443"}, &out); err != nil {
		t.Fatalf("applyAgentFlags error: %v", err)
	}
	if got := os.Getenv("SERVER_URL"); got != "https://example:443" {
		t.Fatalf("SERVER_URL = %q, want %q", got, "https://example:443")
	}

	t.Setenv("SERVER_URL", "")
	out.Reset()
	if err := applyAgentFlags([]string{"-a=example:8080"}, &out); err != nil {
		t.Fatalf("applyAgentFlags error: %v", err)
	}
	if got := os.Getenv("SERVER_URL"); got != "http://example:8080" {
		t.Fatalf("SERVER_URL = %q, want %q", got, "http://example:8080")
	}
}

func TestApplyAgentFlags_IgnoresNonPositive(t *testing.T) {
	t.Setenv("REPORT_INTERVAL", "")
	t.Setenv("POLL_INTERVAL", "")
	var out bytes.Buffer

	if err := applyAgentFlags([]string{"-r=0", "-p=-1"}, &out); err != nil {
		t.Fatalf("applyAgentFlags error: %v", err)
	}
	if got := os.Getenv("REPORT_INTERVAL"); got != "" {
		t.Fatalf("REPORT_INTERVAL = %q, want empty (ignored)", got)
	}
	if got := os.Getenv("POLL_INTERVAL"); got != "" {
		t.Fatalf("POLL_INTERVAL = %q, want empty (ignored)", got)
	}
}

func TestApplyAgentFlags_UnknownFlag(t *testing.T) {
	var out bytes.Buffer
	err := applyAgentFlags([]string{"-x"}, &out)
	if err == nil {
		t.Fatalf("expected error for unknown flag, got nil")
	}
	if want := "flag provided but not defined"; !bytes.Contains(out.Bytes(), []byte(want)) && !bytes.Contains([]byte(err.Error()), []byte(want)) {
		t.Fatalf("error does not mention unknown flag: %v", err)
	}
}
