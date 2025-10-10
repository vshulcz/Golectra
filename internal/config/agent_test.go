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
			},
		},
		{
			name: "env override flags",
			args: []string{"-a", "https://srv.example.com:9090", "-r", "7", "-p", "4", "-k", "hello", "-l", "5"},
			env: map[string]string{
				"ADDRESS":         "https://env-ignored:1234",
				"REPORT_INTERVAL": "99s",
				"POLL_INTERVAL":   "77s",
				"KEY":             "world",
				"RATE_LIMIT":      "3",
			},
			want: AgentConfig{
				Address:        "https://env-ignored:1234",
				ReportInterval: 99 * time.Second,
				PollInterval:   77 * time.Second,
				Key:            "world",
				RateLimit:      3,
			},
		},
		{
			name: "only flags",
			args: []string{"-a", "https://srv.example.com:9090", "-r", "7", "-p", "4", "-k", "hello", "-l", "5"},
			env:  map[string]string{},
			want: AgentConfig{
				Address:        "https://srv.example.com:9090",
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
				"REPORT_INTERVAL": "15s",
				"POLL_INTERVAL":   "3s",
			},
			want: AgentConfig{
				Address:        "https://api.example.com:1234",
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
			for _, k := range []string{"ADDRESS", "REPORT_INTERVAL", "POLL_INTERVAL"} {
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
			if got.ReportInterval != tt.want.ReportInterval {
				t.Errorf("ReportInterval: want %v, got %v", tt.want.ReportInterval, got.ReportInterval)
			}
			if got.PollInterval != tt.want.PollInterval {
				t.Errorf("PollInterval: want %v, got %v", tt.want.PollInterval, got.PollInterval)
			}
		})
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
