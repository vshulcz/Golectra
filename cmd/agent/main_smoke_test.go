package main

import (
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/infra/config"
)

func TestAgentConfigIncludesGRPCAddress(t *testing.T) {
	cfg := config.AgentConfig{
		Address:        "http://localhost:8080",
		GRPCAddress:    "127.0.0.1:9090",
		PollInterval:   1 * time.Second,
		ReportInterval: 1 * time.Second,
		RateLimit:      1,
	}
	if cfg.GRPCAddress == "" {
		t.Fatal("expected GRPCAddress to be set")
	}
}
