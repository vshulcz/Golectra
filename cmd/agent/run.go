package main

import (
	"log"
	"time"

	"github.com/vshulcz/Golectra/internal/agent"
	"github.com/vshulcz/Golectra/internal/misc"
)

func run(newAgent func(agent.Config) interface {
	Start()
	Stop()
}) {
	cfg := agent.Config{
		ServerURL:      misc.Getenv("SERVER_URL", "http://localhost:8080"),
		PollInterval:   misc.GetDuration("POLL_INTERVAL", 2*time.Second),
		ReportInterval: misc.GetDuration("REPORT_INTERVAL", 10*time.Second),
	}

	a := newAgent(cfg)
	log.Printf("agent started: server=%s poll=%s report=%s",
		cfg.ServerURL, cfg.PollInterval, cfg.ReportInterval)

	a.Start()
}
