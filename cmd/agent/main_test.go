package main

import (
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/application/agent"
	"github.com/vshulcz/Golectra/internal/infra/config"
)

func TestBuildVariablesExist(t *testing.T) {
	_ = buildVersion
	_ = buildDate
	_ = buildCommit
}

func TestMapAgentConfig(t *testing.T) {
	cfg := config.AgentConfig{
		PollInterval:   2 * time.Second,
		ReportInterval: 5 * time.Second,
		RateLimit:      3,
	}
	got := mapAgentConfig(cfg)
	want := agent.Config{
		PollInterval:   2 * time.Second,
		ReportInterval: 5 * time.Second,
		RateLimit:      3,
	}
	if got != want {
		t.Fatalf("mapAgentConfig=%+v want %+v", got, want)
	}
}
