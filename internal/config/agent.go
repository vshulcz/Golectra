package config

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

const (
	defaultServerAddr     = "http://localhost:8080"
	defaultReportInterval = 10
	defaultPollInterval   = 2
	defaultRateLimit      = 1
)

type AgentConfig struct {
	Address        string
	Key            string
	PollInterval   time.Duration
	ReportInterval time.Duration
	RateLimit      int
}

// ENV > CLI > defaults
func LoadAgentConfig(args []string, out io.Writer) (AgentConfig, error) {
	if out == nil {
		out = io.Discard
	}

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(out)

	var addrOpt string
	var keyOpt string
	var reportOpt int
	var pollOpt int
	var limitOpt int

	fs.StringVar(&addrOpt, "a", "", fmt.Sprintf("server address (host:port or URL), default: %s", defaultServerAddr))
	fs.StringVar(&keyOpt, "k", "", "secret key for HashSHA256 header")
	fs.IntVar(&reportOpt, "r", 0, fmt.Sprintf("report interval in seconds, default: %d", defaultReportInterval))
	fs.IntVar(&pollOpt, "p", 0, fmt.Sprintf("poll interval in seconds, default: %d", defaultPollInterval))
	fs.IntVar(&limitOpt, "l", 0, "rate limit (max concurrent outgoing requests), default: 1")

	if err := fs.Parse(args); err != nil {
		return AgentConfig{}, err
	}

	addr := FromEnvOrFlag("ADDRESS", addrOpt, defaultServerAddr)
	addr = normalizeAddressURL(addr)
	if _, err := url.ParseRequestURI(addr); err != nil {
		return AgentConfig{}, fmt.Errorf("invalid server address: %q", addr)
	}

	key := FromEnvOrFlag("KEY", keyOpt, "")

	report, _ := FromEnvOrFlagDuration("REPORT_INTERVAL", reportOpt, 0, defaultReportInterval)
	if report <= 0 {
		return AgentConfig{}, fmt.Errorf("report interval must be > 0, got %v", report)
	}

	poll, _ := FromEnvOrFlagDuration("POLL_INTERVAL", pollOpt, 0, defaultPollInterval)
	if poll <= 0 {
		return AgentConfig{}, fmt.Errorf("poll interval must be > 0, got %v", poll)
	}

	limit := FromEnvOrFlagInt("RATE_LIMIT", limitOpt, defaultRateLimit, 1)

	return AgentConfig{
		Address:        addr,
		Key:            key,
		PollInterval:   poll,
		ReportInterval: report,
		RateLimit:      limit,
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
