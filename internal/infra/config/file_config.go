package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type agentFileConfig struct {
	Address        *string `json:"address"`
	ReportInterval *string `json:"report_interval"`
	PollInterval   *string `json:"poll_interval"`
	CryptoKey      *string `json:"crypto_key"`
	Key            *string `json:"key"`
	RateLimit      *int    `json:"rate_limit"`
}

type serverFileConfig struct {
	Address       *string `json:"address"`
	Restore       *bool   `json:"restore"`
	StoreInterval *string `json:"store_interval"`
	StoreFile     *string `json:"store_file"`
	DatabaseDSN   *string `json:"database_dsn"`
	CryptoKey     *string `json:"crypto_key"`
	Key           *string `json:"key"`
	AuditFile     *string `json:"audit_file"`
	AuditURL      *string `json:"audit_url"`
}

func loadAgentFileConfig(path string) (agentFileConfig, error) {
	var cfg agentFileConfig
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}
	if err := loadJSONConfig(path, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func loadServerFileConfig(path string) (serverFileConfig, error) {
	var cfg serverFileConfig
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}
	if err := loadJSONConfig(path, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func loadJSONConfig(path string, dest any) error {
	raw, err := readConfigFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	return nil
}

func parseDurationString(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return d, nil
}

func readConfigFile(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(abs)
	name := filepath.Base(abs)
	fsys := os.DirFS(dir)
	if !fs.ValidPath(name) {
		return nil, fmt.Errorf("invalid config filename: %q", name)
	}
	return fs.ReadFile(fsys, name)
}
