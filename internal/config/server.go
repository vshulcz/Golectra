package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	defaultListenAndServeAddr = ":8080"
	defaultFilePath           = "metrics-db.json"
	defaultDSN                = ""
	defaultStoreInterval      = 300
	defaultRestore            = false
)

// ServerConfig describes how the HTTP server listens, stores data, and emits audit logs.
type ServerConfig struct {
	Address   string
	File      string
	DSN       string
	Key       string
	Interval  time.Duration
	Restore   bool
	AuditFile string
	AuditURL  string
}

// LoadServerConfig resolves CLI flags, environment variables, and defaults (ENV > CLI > defaults).
func LoadServerConfig(args []string, out io.Writer) (ServerConfig, error) {
	if out == nil {
		out = io.Discard
	}

	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(out)

	var addrOpt string
	var fileOpt string
	var dsnOpt string
	var keyOpt string
	var ivalOpt int
	var restoreOpt bool
	var auditFileOpt string
	var auditURLOpt string

	fs.StringVar(&addrOpt, "a", "", fmt.Sprintf("HTTP listen address, default: %s", defaultListenAndServeAddr))
	fs.StringVar(&fileOpt, "f", "", fmt.Sprintf("FILE_STORAGE_PATH, default: %s", defaultFilePath))
	fs.StringVar(&dsnOpt, "d", "", fmt.Sprintf("DATABASE_DSN for Postgres, default: %s", defaultDSN))
	fs.StringVar(&keyOpt, "k", "", "secret key for HashSHA256")
	fs.IntVar(&ivalOpt, "i", -1, fmt.Sprintf("STORE_INTERVAL seconds (0 - sync), default: %d", defaultStoreInterval))
	fs.BoolVar(&restoreOpt, "r", false, fmt.Sprintf("RESTORE on start (true/false), default: %t", defaultRestore))
	fs.StringVar(&auditFileOpt, "audit-file", "", "path to audit log file (disabled if empty)")
	fs.StringVar(&auditURLOpt, "audit-url", "", "URL for sending audit events via HTTP POST (disabled if empty)")

	if err := fs.Parse(args); err != nil {
		return ServerConfig{}, err
	}

	addr := FromEnvOrFlag("ADDRESS", addrOpt, defaultListenAndServeAddr)
	addr = normalizeListenAndServeURL(addr)
	if _, port, err := net.SplitHostPort(addr); err != nil || port == "" {
		return ServerConfig{}, fmt.Errorf("invalid listen address: %q", addr)
	}

	file := FromEnvOrFlag("FILE_STORAGE_PATH", fileOpt, defaultFilePath)
	dsn := FromEnvOrFlag("DATABASE_DSN", dsnOpt, "")
	key := FromEnvOrFlag("KEY", keyOpt, "")
	auditFile := FromEnvOrFlag("AUDIT_FILE", auditFileOpt, "")
	auditURL := FromEnvOrFlag("AUDIT_URL", auditURLOpt, "")

	interval, _ := FromEnvOrFlagDuration("STORE_INTERVAL", ivalOpt, -1, defaultStoreInterval)
	if interval < 0 {
		return ServerConfig{}, fmt.Errorf("store interval must be >= 0, got %v", interval)
	}

	restore := FromEnvOrFlagBool("RESTORE", restoreOpt, defaultRestore)

	return ServerConfig{
		Address:   addr,
		File:      file,
		DSN:       dsn,
		Key:       key,
		Interval:  interval,
		Restore:   restore,
		AuditFile: auditFile,
		AuditURL:  auditURL,
	}, nil
}

func normalizeListenAndServeURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ":8080"
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if u, err := url.Parse(s); err == nil && u.Host != "" {
			return u.Host
		}
	}
	if !strings.Contains(s, ":") {
		return ":" + s
	}
	return s
}
