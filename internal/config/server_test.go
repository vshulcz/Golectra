package config

import (
	"strings"
	"testing"
	"time"
)

func ds(sec int) time.Duration { return time.Duration(sec) * time.Second }

func TestLoadServerConfig(t *testing.T) {
	tests := []struct {
		env     map[string]string
		name    string
		wantErr string
		args    []string
		want    ServerConfig
	}{
		{
			name: "defaults",
			args: []string{},
			env:  map[string]string{},
			want: ServerConfig{
				Address:   defaultListenAndServeAddr,
				File:      defaultFilePath,
				Interval:  ds(defaultStoreInterval),
				Restore:   defaultRestore,
				AuditFile: "",
				AuditURL:  "",
			},
		},
		{
			name: "env override flags",
			args: []string{"-a", "http://127.0.0.1:9090", "-i", "42", "-f", "flags.json", "-r", "-audit-file", "flags.log"},
			env: map[string]string{
				"ADDRESS":           "0.0.0.0:1234",
				"STORE_INTERVAL":    "777s",
				"FILE_STORAGE_PATH": "env.json",
				"RESTORE":           "false",
				"AUDIT_FILE":        "env-audit.log",
				"AUDIT_URL":         "https://audit.example.com",
			},
			want: ServerConfig{
				Address:   "0.0.0.0:1234",
				File:      "env.json",
				Interval:  777 * time.Second,
				Restore:   false,
				AuditFile: "env-audit.log",
				AuditURL:  "https://audit.example.com",
			},
		},
		{
			name: "env (no flags)",
			args: []string{},
			env: map[string]string{
				"ADDRESS":           "http://0.0.0.0:5050",
				"STORE_INTERVAL":    "15s",
				"FILE_STORAGE_PATH": "from-env.json",
				"RESTORE":           "true",
				"AUDIT_FILE":        "/tmp/audit.log",
			},
			want: ServerConfig{
				Address:   "0.0.0.0:5050",
				File:      "from-env.json",
				Interval:  15 * time.Second,
				Restore:   true,
				AuditFile: "/tmp/audit.log",
				AuditURL:  "",
			},
		},
		{
			name:    "invalid listen address: URL without port",
			args:    []string{"-a", "http://example.com"},
			wantErr: "invalid listen address",
		},
		{
			name:    "invalid listen address: IPv6 without port",
			args:    []string{"-a", "http://[::1]"},
			wantErr: "invalid listen address",
		},
		{
			name: "interval == 0 via flag is allowed (sync mode)",
			args: []string{"-i", "0", "-audit-url", "https://audit"},
			env:  map[string]string{},
			want: ServerConfig{
				Address:   defaultListenAndServeAddr,
				File:      defaultFilePath,
				Interval:  0,
				Restore:   defaultRestore,
				AuditFile: "",
				AuditURL:  "https://audit",
			},
		},
		{
			name: "restore comes from ENV",
			args: []string{},
			env:  map[string]string{"RESTORE": "true"},
			want: ServerConfig{
				Address:   defaultListenAndServeAddr,
				File:      defaultFilePath,
				Interval:  ds(defaultStoreInterval),
				Restore:   true,
				AuditFile: "",
				AuditURL:  "",
			},
		},
		{
			name: "address accepts plain port (normalized to :port)",
			args: []string{"-a", "9090"},
			want: ServerConfig{
				Address:   ":9090",
				File:      defaultFilePath,
				Interval:  ds(defaultStoreInterval),
				Restore:   defaultRestore,
				AuditFile: "",
				AuditURL:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"ADDRESS", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE", "AUDIT_FILE", "AUDIT_URL"} {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got, err := LoadServerConfig(tt.args, nil)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Address != tt.want.Address {
				t.Errorf("Address: want %q, got %q", tt.want.Address, got.Address)
			}
			if got.File != tt.want.File {
				t.Errorf("File: want %q, got %q", tt.want.File, got.File)
			}
			if got.Interval != tt.want.Interval {
				t.Errorf("Interval: want %v, got %v", tt.want.Interval, got.Interval)
			}
			if got.Restore != tt.want.Restore {
				t.Errorf("Restore: want %v, got %v", tt.want.Restore, got.Restore)
			}
			if got.AuditFile != tt.want.AuditFile {
				t.Errorf("AuditFile: want %q, got %q", tt.want.AuditFile, got.AuditFile)
			}
			if got.AuditURL != tt.want.AuditURL {
				t.Errorf("AuditURL: want %q, got %q", tt.want.AuditURL, got.AuditURL)
			}
		})
	}
}

func TestNormalizeListenAndServeURL(t *testing.T) {
	cases := map[string]string{
		"":                        ":8080",
		"   ":                     ":8080",
		"8080":                    ":8080",
		" 9090 ":                  ":9090",
		":8081":                   ":8081",
		"0.0.0.0:9090":            "0.0.0.0:9090",
		"http://0.0.0.0:9090":     "0.0.0.0:9090",
		"https://example.com:443": "example.com:443",
		"http://example.com":      "example.com",
		"[::1]:8080":              "[::1]:8080",
	}

	for in, want := range cases {
		if got := normalizeListenAndServeURL(in); got != want {
			t.Errorf("normalizeListenAndServeURL(%q): want %q, got %q", in, want, got)
		}
	}
}
