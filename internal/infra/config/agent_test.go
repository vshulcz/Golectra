package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func d(sec int) time.Duration { return time.Duration(sec) * time.Second }

func TestLoadAgentConfig(t *testing.T) {
	tests := []struct {
		env       map[string]string
		name      string
		wantError string
		args      []string
		want      AgentConfig
	}{
		{
			name: "defaults",
			args: []string{},
			env:  map[string]string{},
			want: AgentConfig{
				Address:        defaultServerAddr,
				ReportInterval: d(defaultReportInterval),
				PollInterval:   d(defaultPollInterval),
				Key:            "",
				GRPCAddress:    "",
			},
		},
		{
			name: "env override flags",
			args: []string{"-a", "https://srv.example.com:9090", "-g", "localhost:9091", "-r", "7", "-p", "4", "-k", "hello", "-l", "5"},
			env: map[string]string{
				"ADDRESS":         "https://env-ignored:1234",
				"GRPC_ADDRESS":    "127.0.0.1:7777",
				"REPORT_INTERVAL": "99s",
				"POLL_INTERVAL":   "77s",
				"KEY":             "world",
				"RATE_LIMIT":      "3",
			},
			want: AgentConfig{
				Address:        "https://env-ignored:1234",
				GRPCAddress:    "127.0.0.1:7777",
				ReportInterval: 99 * time.Second,
				PollInterval:   77 * time.Second,
				Key:            "world",
				RateLimit:      3,
			},
		},
		{
			name: "only flags",
			args: []string{"-a", "https://srv.example.com:9090", "-g", ":7777", "-r", "7", "-p", "4", "-k", "hello", "-l", "5"},
			env:  map[string]string{},
			want: AgentConfig{
				Address:        "https://srv.example.com:9090",
				GRPCAddress:    "localhost:7777",
				ReportInterval: 7 * time.Second,
				PollInterval:   4 * time.Second,
				Key:            "hello",
				RateLimit:      5,
			},
		},
		{
			name: "env fallback",
			args: []string{},
			env: map[string]string{
				"ADDRESS":         "https://api.example.com:1234",
				"GRPC_ADDRESS":    "9099",
				"REPORT_INTERVAL": "15s",
				"POLL_INTERVAL":   "3s",
			},
			want: AgentConfig{
				Address:        "https://api.example.com:1234",
				GRPCAddress:    "localhost:9099",
				ReportInterval: 15 * time.Second,
				PollInterval:   3 * time.Second,
			},
		},
		{
			name: "invalid report interval from env",
			args: []string{},
			env: map[string]string{
				"REPORT_INTERVAL": "-1s",
				"POLL_INTERVAL":   "2s",
			},
			wantError: "report interval must be > 0",
		},
		{
			name: "invalid poll interval from env",
			args: []string{},
			env: map[string]string{
				"REPORT_INTERVAL": "1s",
				"POLL_INTERVAL":   "0s",
			},
			wantError: "poll interval must be > 0",
		},
		{
			name:      "flag parse error",
			args:      []string{"-r", "oops"},
			env:       map[string]string{},
			wantError: "invalid value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"ADDRESS", "GRPC_ADDRESS", "REPORT_INTERVAL", "POLL_INTERVAL", "CRYPTO_KEY", "CONFIG", "KEY", "RATE_LIMIT"} {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got, err := LoadAgentConfig(tt.args, os.Stderr)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantError)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error %q, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Address != tt.want.Address {
				t.Errorf("Address: want %q, got %q", tt.want.Address, got.Address)
			}
			if got.GRPCAddress != tt.want.GRPCAddress {
				t.Errorf("GRPCAddress: want %q, got %q", tt.want.GRPCAddress, got.GRPCAddress)
			}
			if got.ReportInterval != tt.want.ReportInterval {
				t.Errorf("ReportInterval: want %v, got %v", tt.want.ReportInterval, got.ReportInterval)
			}
			if got.PollInterval != tt.want.PollInterval {
				t.Errorf("PollInterval: want %v, got %v", tt.want.PollInterval, got.PollInterval)
			}
		})
	}
}

func TestLoadAgentConfig_FileConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/agent.json"
	cfg := `{"address":"http://cfg:8080","grpc_address":"localhost:9100","report_interval":"3s","poll_interval":"4s","crypto_key":"pub.pem","key":"hmac","rate_limit":7}`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CONFIG", path)
	t.Setenv("ADDRESS", "")
	t.Setenv("REPORT_INTERVAL", "")
	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("GRPC_ADDRESS", "")
	t.Setenv("CRYPTO_KEY", "")
	t.Setenv("KEY", "")
	t.Setenv("RATE_LIMIT", "")

	got, err := LoadAgentConfig([]string{}, os.Stderr)
	if err != nil {
		t.Fatalf("LoadAgentConfig error: %v", err)
	}

	if got.Address != "http://cfg:8080" {
		t.Fatalf("Address=%q want %q", got.Address, "http://cfg:8080")
	}
	if got.GRPCAddress != "localhost:9100" {
		t.Fatalf("GRPCAddress=%q want %q", got.GRPCAddress, "localhost:9100")
	}
	if got.ReportInterval != 3*time.Second {
		t.Fatalf("ReportInterval=%v want %v", got.ReportInterval, 3*time.Second)
	}
	if got.PollInterval != 4*time.Second {
		t.Fatalf("PollInterval=%v want %v", got.PollInterval, 4*time.Second)
	}
	if got.CryptoKey != "pub.pem" || got.Key != "hmac" || got.RateLimit != 7 {
		t.Fatalf("config mismatch: %+v", got)
	}
}

func TestLoadAgentConfig_FileConfig_Priority(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/agent.json"
	cfg := `{"address":"http://cfg:8080","report_interval":"3s","poll_interval":"4s"}`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("CONFIG", path)
	t.Setenv("ADDRESS", "http://env:9090")
	t.Setenv("REPORT_INTERVAL", "9s")

	got, err := LoadAgentConfig([]string{"-p", "7"}, os.Stderr)
	if err != nil {
		t.Fatalf("LoadAgentConfig error: %v", err)
	}
	if got.Address != "http://env:9090" {
		t.Fatalf("Address=%q want %q", got.Address, "http://env:9090")
	}
	if got.ReportInterval != 9*time.Second {
		t.Fatalf("ReportInterval=%v want %v", got.ReportInterval, 9*time.Second)
	}
	if got.PollInterval != 7*time.Second {
		t.Fatalf("PollInterval=%v want %v", got.PollInterval, 7*time.Second)
	}
}

func TestNormalizeAddressURL(t *testing.T) {
	cases := map[string]string{
		"":                   "http://localhost:8080",
		"   ":                "http://localhost:8080",
		"example.com:9999":   "http://example.com:9999",
		"localhost:8000":     "http://localhost:8000",
		":8081":              "http://localhost:8081",
		"  :8081  ":          "http://localhost:8081",
		"http://ex.com:80":   "http://ex.com:80",
		"https://ex.com:443": "https://ex.com:443",
		"://bad":             "http://localhost://bad",
	}
	for in, want := range cases {
		if got := normalizeAddressURL(in); got != want {
			t.Errorf("normalizeAddressURL(%q): want %q, got %q", in, want, got)
		}
	}
}

func TestNormalizeGRPCAddress(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"   ":                 "",
		"9090":                "localhost:9090",
		":9090":               "localhost:9090",
		"example.com:3200":    "example.com:3200",
		"http://host:3200":    "host:3200",
		"https://host:8443":   "host:8443",
		"  localhost:50051  ": "localhost:50051",
		"bad://value":         "bad://value",
	}
	for in, want := range cases {
		if got := normalizeGRPCAddress(in); got != want {
			t.Errorf("normalizeGRPCAddress(%q): want %q, got %q", in, want, got)
		}
	}
}
