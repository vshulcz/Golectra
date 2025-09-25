package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/internal/misc"
)

const (
	defaultListenAndServeAddr = ":8080"
	defaultFilePath           = "metrics-db.json"
	defaultDSN                = ""
	defaultStoreInterval      = 300
	defaultRestore            = false
)

type ServerConfig struct {
	Address  string
	File     string
	DSN      string
	Interval time.Duration
	Restore  bool
}

// CLI > ENV > defaults
func LoadServerConfig(args []string, out io.Writer) (ServerConfig, error) {
	if out == nil {
		out = io.Discard
	}

	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(out)

	var addrOpt string
	var fileOpt string
	var dsnOpt string
	var ivalOpt int
	var restoreOpt bool

	fs.StringVar(&addrOpt, "a", "", fmt.Sprintf("HTTP listen address, default: %s", defaultListenAndServeAddr))
	fs.StringVar(&fileOpt, "f", "", fmt.Sprintf("FILE_STORAGE_PATH, default: %s", defaultFilePath))
	fs.StringVar(&dsnOpt, "d", "", fmt.Sprintf("DATABASE_DSN for Postgres, default: %s", defaultDSN))
	fs.IntVar(&ivalOpt, "i", -1, fmt.Sprintf("STORE_INTERVAL seconds (0 - sync), default: %d", defaultStoreInterval))
	fs.BoolVar(&restoreOpt, "r", false, fmt.Sprintf("RESTORE on start (true/false), default: %t", defaultRestore))

	if err := fs.Parse(args); err != nil {
		return ServerConfig{}, err
	}

	addr := addrOpt
	if strings.TrimSpace(addr) == "" {
		addr = misc.Getenv("ADDRESS", defaultListenAndServeAddr)
	}
	addr = normalizeListenAndServeURL(addr)

	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return ServerConfig{}, fmt.Errorf("invalid listen address: %q", addr)
	}

	file := fileOpt
	if strings.TrimSpace(file) == "" {
		file = misc.Getenv("FILE_STORAGE_PATH", defaultFilePath)
	}

	dsn := misc.Getenv("DATABASE_DSN", defaultDSN)
	if dsn == "" {
		dsn = strings.TrimSpace(dsnOpt)
	}

	var interval time.Duration
	if ivalOpt >= 0 {
		interval = time.Duration(ivalOpt) * time.Second
	} else {
		interval = misc.GetDuration("STORE_INTERVAL", time.Duration(defaultStoreInterval)*time.Second)
	}

	restore := restoreOpt
	if !restore {
		restore = misc.GetBool("RESTORE", defaultRestore)
	}

	return ServerConfig{
		Address:  addr,
		File:     file,
		DSN:      dsn,
		Interval: interval,
		Restore:  restore,
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
