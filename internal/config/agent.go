package config

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/internal/misc"
)

const (
	defaultServerAddr     = "http://localhost:8080"
	defaultReportInterval = 10
	defaultPollInterval   = 2
)

type AgentConfig struct {
	Address        string
	PollInterval   time.Duration
	ReportInterval time.Duration
}

// CLI > ENV > defaults
func LoadAgentConfig(args []string, out io.Writer) (AgentConfig, error) {
	if out == nil {
		out = io.Discard
	}

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(out)

	var addrOpt string
	var reportOpt int
	var pollOpt int

	fs.StringVar(&addrOpt, "a", "", fmt.Sprintf("server address (host:port or URL), default: %s", defaultServerAddr))
	fs.IntVar(&reportOpt, "r", 0, fmt.Sprintf("report interval in seconds, default: %d", defaultReportInterval))
	fs.IntVar(&pollOpt, "p", 0, fmt.Sprintf("poll interval in seconds, default: %d", defaultPollInterval))

	if err := fs.Parse(args); err != nil {
		return AgentConfig{}, err
	}

	addr := addrOpt
	if strings.TrimSpace(addr) == "" {
		addr = misc.Getenv("ADDRESS", defaultServerAddr)
	}
	addr = normalizeAddressURL(addr)

	if _, err := url.ParseRequestURI(addr); err != nil {
		return AgentConfig{}, fmt.Errorf("invalid server address: %q", addr)
	}

	var report time.Duration
	if reportOpt > 0 {
		report = time.Duration(reportOpt) * time.Second
	} else {
		report = misc.GetDuration("REPORT_INTERVAL", time.Duration(defaultReportInterval)*time.Second)
	}

	if report <= 0 {
		return AgentConfig{}, fmt.Errorf("report interval must be > 0, got %v", report)
	}

	var poll time.Duration
	if pollOpt > 0 {
		poll = time.Duration(pollOpt) * time.Second
	} else {
		poll = misc.GetDuration("POLL_INTERVAL", time.Duration(defaultPollInterval)*time.Second)
	}

	if poll <= 0 {
		return AgentConfig{}, fmt.Errorf("poll interval must be > 0, got %v", poll)
	}

	return AgentConfig{
		Address:        addr,
		PollInterval:   poll,
		ReportInterval: report,
	}, nil
}

func normalizeAddressURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultServerAddr
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	if strings.HasPrefix(s, ":") {
		return "http://localhost" + s
	}
	return "http://" + s
}
