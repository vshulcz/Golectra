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

func TestAgentConfigAlias(t *testing.T) {
	cfg := config.AgentConfig{
		Address:        "http://localhost:8080",
		Key:            "k",
		CryptoKey:      "pub.pem",
		PollInterval:   2 * time.Second,
		ReportInterval: 5 * time.Second,
		RateLimit:      3,
	}
	var got agent.Config = cfg
	if got != cfg {
		t.Fatalf("agent.Config=%+v want %+v", got, cfg)
	}
}
