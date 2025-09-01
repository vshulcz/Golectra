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
		ServerURL:      misc.Getenv("SERVER_URL", DefaultServerAddr),
		PollInterval:   misc.GetDuration("POLL_INTERVAL", DefaultPollInterval*time.Second),
		ReportInterval: misc.GetDuration("REPORT_INTERVAL", DefaultReportInterval*time.Second),
	}

	a := newAgent(cfg)
	log.Printf("agent started: server=%s poll=%s report=%s",
		cfg.ServerURL, cfg.PollInterval, cfg.ReportInterval)

	a.Start()
}
