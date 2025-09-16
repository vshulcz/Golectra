package main

import (
	"log"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/internal/agent"
	"github.com/vshulcz/Golectra/internal/misc"
)

func run(newAgent func(agent.Config) interface {
	Start()
	Stop()
}) {
	addr := misc.Getenv("ADDRESS", DefaultServerAddr)
	cfg := agent.Config{
		Address:        normalizeURL(addr),
		PollInterval:   misc.GetDuration("POLL_INTERVAL", DefaultPollInterval*time.Second),
		ReportInterval: misc.GetDuration("REPORT_INTERVAL", DefaultReportInterval*time.Second),
	}

	a := newAgent(cfg)
	log.Printf("agent started: server=%s poll=%s report=%s",
		cfg.Address, cfg.PollInterval, cfg.ReportInterval)

	a.Start()
}

func normalizeURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "http://localhost:8080"
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	if strings.HasPrefix(s, ":") {
		return "http://localhost" + s
	}
	return "http://" + s
}
