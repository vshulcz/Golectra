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
	file := resolveServerFile(flags, fileCfg)
	dsn := resolveServerDSN(flags, fileCfg)
	key := resolveServerKey(flags, fileCfg)
	cryptoKey := resolveServerCryptoKey(flags, fileCfg)
	auditFile := resolveServerAuditFile(flags, fileCfg)
	auditURL := resolveServerAuditURL(flags, fileCfg)
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
	fileAddr := defaultListenAndServeAddr
	if fileCfg.Address != nil {
		fileAddr = *fileCfg.Address
	}
	addr := FromEnvOrFlag("ADDRESS", flags.addrOpt, fileAddr)
	addr = normalizeListenAndServeURL(addr)
	if _, port, err := net.SplitHostPort(addr); err != nil || port == "" {
		return "", fmt.Errorf("invalid listen address: %q", addr)
	}
	return addr, nil
}

func resolveServerFile(flags serverFlagOptions, fileCfg serverFileConfig) string {
	fileDef := defaultFilePath
	if fileCfg.StoreFile != nil {
		fileDef = *fileCfg.StoreFile
	}
	file := fileDef
	if v := strings.TrimSpace(flags.fileOpt); v != "" {
		file = v
	}
	if v := strings.TrimSpace(os.Getenv("FILE_STORAGE_PATH")); v != "" {
		file = v
	}
	if v := strings.TrimSpace(os.Getenv("STORE_FILE")); v != "" {
		file = v
	}
	return file
}

func resolveServerDSN(flags serverFlagOptions, fileCfg serverFileConfig) string {
	fileDSN := ""
	if fileCfg.DatabaseDSN != nil {
		fileDSN = *fileCfg.DatabaseDSN
	}
	return FromEnvOrFlag("DATABASE_DSN", flags.dsnOpt, fileDSN)
}

func resolveServerKey(flags serverFlagOptions, fileCfg serverFileConfig) string {
	fileKey := ""
	if fileCfg.Key != nil {
		fileKey = *fileCfg.Key
	}
	return FromEnvOrFlag("KEY", flags.keyOpt, fileKey)
}

func resolveServerCryptoKey(flags serverFlagOptions, fileCfg serverFileConfig) string {
	fileCryptoKey := ""
	if fileCfg.CryptoKey != nil {
		fileCryptoKey = *fileCfg.CryptoKey
	}
	return FromEnvOrFlag("CRYPTO_KEY", flags.cryptoKeyOpt, fileCryptoKey)
}

func resolveServerAuditFile(flags serverFlagOptions, fileCfg serverFileConfig) string {
	fileAuditFile := ""
	if fileCfg.AuditFile != nil {
		fileAuditFile = *fileCfg.AuditFile
	}
	return FromEnvOrFlag("AUDIT_FILE", flags.auditFileOpt, fileAuditFile)
}

func resolveServerAuditURL(flags serverFlagOptions, fileCfg serverFileConfig) string {
	fileAuditURL := ""
	if fileCfg.AuditURL != nil {
		fileAuditURL = *fileCfg.AuditURL
	}
	return FromEnvOrFlag("AUDIT_URL", flags.auditURLOpt, fileAuditURL)
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
