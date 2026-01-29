package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	agentsvc "github.com/vshulcz/Golectra/internal/application/agent"
)

const (
	defaultServerAddr     = "http://localhost:8080"
	defaultReportInterval = 10
	defaultPollInterval   = 2
	defaultRateLimit      = 1
)

// AgentConfig holds runtime parameters for the metrics agent.
type AgentConfig = agentsvc.Config

// LoadAgentConfig resolves CLI flags, environment variables, and defaults (ENV > CLI > defaults).
func LoadAgentConfig(args []string, out io.Writer) (AgentConfig, error) {
	if out == nil {
		out = io.Discard
	}

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(out)

	var addrOpt string
	var grpcAddrOpt string
	var keyOpt string
	var cryptoKeyOpt string
	var configOpt string
	var reportOpt int
	var pollOpt int
	var limitOpt int

	fs.StringVar(&addrOpt, "a", "", fmt.Sprintf("server address (host:port or URL), default: %s", defaultServerAddr))
	fs.StringVar(&grpcAddrOpt, "g", "", "gRPC server address (host:port)")
	fs.StringVar(&grpcAddrOpt, "grpc-address", "", "gRPC server address (host:port)")
	fs.StringVar(&keyOpt, "k", "", "secret key for HashSHA256 header")
	fs.StringVar(&cryptoKeyOpt, "crypto-key", "", "path to RSA public key for request encryption")
	fs.StringVar(&configOpt, "c", "", "path to JSON config file")
	fs.StringVar(&configOpt, "config", "", "path to JSON config file")
	fs.IntVar(&reportOpt, "r", 0, fmt.Sprintf("report interval in seconds, default: %d", defaultReportInterval))
	fs.IntVar(&pollOpt, "p", 0, fmt.Sprintf("poll interval in seconds, default: %d", defaultPollInterval))
	fs.IntVar(&limitOpt, "l", 0, "rate limit (max concurrent outgoing requests), default: 1")

	if err := fs.Parse(args); err != nil {
		return AgentConfig{}, err
	}

	cfgPath := FromEnvOrFlag("CONFIG", configOpt, "")
	fileCfg, err := loadAgentFileConfig(cfgPath)
	if err != nil {
		return AgentConfig{}, err
	}

	fileAddr := defaultServerAddr
	if fileCfg.Address != nil {
		fileAddr = *fileCfg.Address
	}
	addr := FromEnvOrFlag("ADDRESS", addrOpt, fileAddr)
	addr = normalizeAddressURL(addr)
	if _, err := url.ParseRequestURI(addr); err != nil {
		return AgentConfig{}, fmt.Errorf("invalid server address: %q", addr)
	}

	fileGRPCAddr := ""
	if fileCfg.GRPCAddress != nil {
		fileGRPCAddr = *fileCfg.GRPCAddress
	}
	grpcAddr := FromEnvOrFlag("GRPC_ADDRESS", grpcAddrOpt, fileGRPCAddr)
	grpcAddr = normalizeGRPCAddress(grpcAddr)
	if grpcAddr != "" {
		if _, _, err := net.SplitHostPort(grpcAddr); err != nil {
			return AgentConfig{}, fmt.Errorf("invalid gRPC server address: %q", grpcAddr)
		}
	}

	fileKey := ""
	if fileCfg.Key != nil {
		fileKey = *fileCfg.Key
	}
	key := FromEnvOrFlag("KEY", keyOpt, fileKey)

	fileCryptoKey := ""
	if fileCfg.CryptoKey != nil {
		fileCryptoKey = *fileCfg.CryptoKey
	}
	cryptoKey := FromEnvOrFlag("CRYPTO_KEY", cryptoKeyOpt, fileCryptoKey)

	fileReport := time.Duration(defaultReportInterval) * time.Second
	if fileCfg.ReportInterval != nil {
		if fileReport, err = parseDurationString(*fileCfg.ReportInterval); err != nil {
			return AgentConfig{}, fmt.Errorf("invalid report_interval: %w", err)
		}
	}
	report, _ := FromEnvOrFlagDurationWithDefault("REPORT_INTERVAL", reportOpt, 0, fileReport)
	if report <= 0 {
		return AgentConfig{}, fmt.Errorf("report interval must be > 0, got %v", report)
	}

	filePoll := time.Duration(defaultPollInterval) * time.Second
	if fileCfg.PollInterval != nil {
		if filePoll, err = parseDurationString(*fileCfg.PollInterval); err != nil {
			return AgentConfig{}, fmt.Errorf("invalid poll_interval: %w", err)
		}
	}
	poll, _ := FromEnvOrFlagDurationWithDefault("POLL_INTERVAL", pollOpt, 0, filePoll)
	if poll <= 0 {
		return AgentConfig{}, fmt.Errorf("poll interval must be > 0, got %v", poll)
	}

	fileLimit := defaultRateLimit
	if fileCfg.RateLimit != nil {
		fileLimit = *fileCfg.RateLimit
	}
	limit := FromEnvOrFlagInt("RATE_LIMIT", limitOpt, fileLimit, 1)

	return AgentConfig{
		Address:        addr,
		GRPCAddress:    grpcAddr,
		Key:            key,
		CryptoKey:      cryptoKey,
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

func normalizeGRPCAddress(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if u, err := url.Parse(s); err == nil && u.Host != "" {
			return u.Host
		}
	}
	if strings.HasPrefix(s, ":") {
		return "localhost" + s
	}
	if !strings.Contains(s, ":") {
		return "localhost:" + s
	}
	return s
}
