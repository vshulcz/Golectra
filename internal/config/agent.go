package config

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/internal/misc"
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

	addr := strings.TrimSpace(misc.Getenv("ADDRESS", ""))
	if addr == "" {
		addr = strings.TrimSpace(addrOpt)
	}
	if addr == "" {
		addr = defaultServerAddr
	}
	addr = normalizeAddressURL(addr)
	if _, err := url.ParseRequestURI(addr); err != nil {
		return AgentConfig{}, fmt.Errorf("invalid server address: %q", addr)
	}

	key := strings.TrimSpace(misc.Getenv("KEY", ""))
	if key == "" {
		key = strings.TrimSpace(keyOpt)
	}

	report := misc.GetDuration("REPORT_INTERVAL", 0)
	if v := report; v == 0 && strings.TrimSpace(misc.Getenv("REPORT_INTERVAL", "")) == "" {
		if reportOpt > 0 {
			report = time.Duration(reportOpt) * time.Second
		} else {
			report = time.Duration(defaultReportInterval) * time.Second
		}
	}
	if report <= 0 {
		return AgentConfig{}, fmt.Errorf("report interval must be > 0, got %v", report)
	}

	poll := misc.GetDuration("POLL_INTERVAL", 0)
	if v := poll; v == 0 && strings.TrimSpace(misc.Getenv("POLL_INTERVAL", "")) == "" {
		if pollOpt > 0 {
			poll = time.Duration(pollOpt) * time.Second
		} else {
			poll = time.Duration(defaultPollInterval) * time.Second
		}
	}
	if poll <= 0 {
		return AgentConfig{}, fmt.Errorf("poll interval must be > 0, got %v", poll)
	}

	limit := defaultRateLimit
	if ev := strings.TrimSpace(misc.Getenv("RATE_LIMIT", "")); ev != "" {
		if n, err := strconv.Atoi(ev); err == nil && n > 0 {
			limit = n
		}
	}
	if limit == defaultRateLimit && limitOpt > 0 {
		limit = limitOpt
	}
	if limit <= 0 {
		limit = defaultRateLimit
	}

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
