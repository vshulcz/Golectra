package main

import (
	"log"
	"os"

	"github.com/vshulcz/Golectra/internal/agent"
	"github.com/vshulcz/Golectra/internal/config"
)

func run(newAgent func(config.AgentConfig) agent.Agent) {
	cfg, err := config.LoadAgentConfig(os.Args[1:], nil)
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}

	a := newAgent(cfg)
	log.Printf("agent started: server=%s poll=%s report=%s",
		cfg.Address, cfg.PollInterval, cfg.ReportInterval)

	a.Start()
}
