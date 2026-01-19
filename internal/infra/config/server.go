package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
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
	CryptoKey string
	Interval  time.Duration
	Restore   bool
	AuditFile string
	AuditURL  string
}

// LoadServerConfig resolves CLI flags, environment variables, and defaults (ENV > CLI > defaults).
func LoadServerConfig(args []string, out io.Writer) (ServerConfig, error) {
	flags, err := parseServerFlags(args, out)
	if err != nil {
		return ServerConfig{}, err
	}

	cfgPath := FromEnvOrFlag("CONFIG", flags.configOpt, "")
	fileCfg, err := loadServerFileConfig(cfgPath)
	if err != nil {
		return ServerConfig{}, err
	}

	return buildServerConfig(flags, fileCfg)
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

type serverFlagOptions struct {
	addrOpt      string
	fileOpt      string
	dsnOpt       string
	keyOpt       string
	cryptoKeyOpt string
	configOpt    string
	ivalOpt      int
	restoreOpt   bool
	auditFileOpt string
	auditURLOpt  string
}

func parseServerFlags(args []string, out io.Writer) (serverFlagOptions, error) {
	if out == nil {
		out = io.Discard
	}
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(out)

	var opts serverFlagOptions
	fs.StringVar(&opts.addrOpt, "a", "", fmt.Sprintf("HTTP listen address, default: %s", defaultListenAndServeAddr))
	fs.StringVar(&opts.fileOpt, "f", "", fmt.Sprintf("FILE_STORAGE_PATH, default: %s", defaultFilePath))
	fs.StringVar(&opts.dsnOpt, "d", "", fmt.Sprintf("DATABASE_DSN for Postgres, default: %s", defaultDSN))
	fs.StringVar(&opts.keyOpt, "k", "", "secret key for HashSHA256")
	fs.StringVar(&opts.cryptoKeyOpt, "crypto-key", "", "path to RSA private key for request decryption")
	fs.StringVar(&opts.configOpt, "c", "", "path to JSON config file")
	fs.StringVar(&opts.configOpt, "config", "", "path to JSON config file")
	fs.IntVar(&opts.ivalOpt, "i", -1, fmt.Sprintf("STORE_INTERVAL seconds (0 - sync), default: %d", defaultStoreInterval))
	fs.BoolVar(&opts.restoreOpt, "r", false, fmt.Sprintf("RESTORE on start (true/false), default: %t", defaultRestore))
	fs.StringVar(&opts.auditFileOpt, "audit-file", "", "path to audit log file (disabled if empty)")
	fs.StringVar(&opts.auditURLOpt, "audit-url", "", "URL for sending audit events via HTTP POST (disabled if empty)")

	if err := fs.Parse(args); err != nil {
		return serverFlagOptions{}, err
	}
	return opts, nil
}

func buildServerConfig(flags serverFlagOptions, fileCfg serverFileConfig) (ServerConfig, error) {
	addr, err := resolveServerAddress(flags, fileCfg)
	if err != nil {
		return ServerConfig{}, err
	}
	file := resolveString([]string{"STORE_FILE", "FILE_STORAGE_PATH"}, flags.fileOpt, fileValue(fileCfg.StoreFile, defaultFilePath))
	dsn := resolveString([]string{"DATABASE_DSN"}, flags.dsnOpt, fileValue(fileCfg.DatabaseDSN, ""))
	key := resolveString([]string{"KEY"}, flags.keyOpt, fileValue(fileCfg.Key, ""))
	cryptoKey := resolveString([]string{"CRYPTO_KEY"}, flags.cryptoKeyOpt, fileValue(fileCfg.CryptoKey, ""))
	auditFile := resolveString([]string{"AUDIT_FILE"}, flags.auditFileOpt, fileValue(fileCfg.AuditFile, ""))
	auditURL := resolveString([]string{"AUDIT_URL"}, flags.auditURLOpt, fileValue(fileCfg.AuditURL, ""))
	interval, err := resolveServerInterval(flags, fileCfg)
	if err != nil {
		return ServerConfig{}, err
	}
	restore := resolveServerRestore(flags, fileCfg)

	return ServerConfig{
		Address:   addr,
		File:      file,
		DSN:       dsn,
		Key:       key,
		CryptoKey: cryptoKey,
		Interval:  interval,
		Restore:   restore,
		AuditFile: auditFile,
		AuditURL:  auditURL,
	}, nil
}

func resolveServerAddress(flags serverFlagOptions, fileCfg serverFileConfig) (string, error) {
	fileAddr := fileValue(fileCfg.Address, defaultListenAndServeAddr)
	addr := resolveString([]string{"ADDRESS"}, flags.addrOpt, fileAddr)
	addr = normalizeListenAndServeURL(addr)
	if _, port, err := net.SplitHostPort(addr); err != nil || port == "" {
		return "", fmt.Errorf("invalid listen address: %q", addr)
	}
	return addr, nil
}

func resolveServerInterval(flags serverFlagOptions, fileCfg serverFileConfig) (time.Duration, error) {
	fileInterval := time.Duration(defaultStoreInterval) * time.Second
	if fileCfg.StoreInterval != nil {
		parsed, err := parseDurationString(*fileCfg.StoreInterval)
		if err != nil {
			return 0, fmt.Errorf("invalid store_interval: %w", err)
		}
		fileInterval = parsed
	}
	interval, _ := FromEnvOrFlagDurationWithDefault("STORE_INTERVAL", flags.ivalOpt, -1, fileInterval)
	if interval < 0 {
		return 0, fmt.Errorf("store interval must be >= 0, got %v", interval)
	}
	return interval, nil
}

func resolveServerRestore(flags serverFlagOptions, fileCfg serverFileConfig) bool {
	fileRestore := defaultRestore
	if fileCfg.Restore != nil {
		fileRestore = *fileCfg.Restore
	}
	return FromEnvOrFlagBool("RESTORE", flags.restoreOpt, fileRestore)
}

func resolveString(envKeys []string, flagVal, fileVal string) string {
	for _, key := range envKeys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	if v := strings.TrimSpace(flagVal); v != "" {
		return v
	}
	return fileVal
}

func fileValue(ptr *string, def string) string {
	if ptr == nil {
		return def
	}
	return *ptr
}
