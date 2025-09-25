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
				Address:  defaultListenAndServeAddr,
				File:     defaultFilePath,
				Interval: ds(defaultStoreInterval),
				Restore:  defaultRestore,
			},
		},
		{
			name: "flags override env",
			args: []string{"-a", "http://127.0.0.1:9090", "-i", "42", "-f", "flags.json", "-r"},
			env: map[string]string{
				"ADDRESS":           "0.0.0.0:1234",
				"STORE_INTERVAL":    "777s",
				"FILE_STORAGE_PATH": "env.json",
				"RESTORE":           "false",
			},
			want: ServerConfig{
				Address:  "127.0.0.1:9090",
				File:     "flags.json",
				Interval: 42 * time.Second,
				Restore:  true,
			},
		},
		{
			name: "env fallback (no flags)",
			args: []string{},
			env: map[string]string{
				"ADDRESS":           "http://0.0.0.0:5050",
				"STORE_INTERVAL":    "15s",
				"FILE_STORAGE_PATH": "from-env.json",
				"RESTORE":           "true",
			},
			want: ServerConfig{
				Address:  "0.0.0.0:5050",
				File:     "from-env.json",
				Interval: 15 * time.Second,
				Restore:  true,
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
			args: []string{"-i", "0"},
			env:  map[string]string{},
			want: ServerConfig{
				Address:  defaultListenAndServeAddr,
				File:     defaultFilePath,
				Interval: 0,
				Restore:  defaultRestore,
			},
		},
		{
			name: "restore comes from ENV when flag not set",
			args: []string{},
			env:  map[string]string{"RESTORE": "true"},
			want: ServerConfig{
				Address:  defaultListenAndServeAddr,
				File:     defaultFilePath,
				Interval: ds(defaultStoreInterval),
				Restore:  true,
			},
		},
		{
			name: "address accepts plain port (normalized to :port)",
			args: []string{"-a", "9090"},
			want: ServerConfig{
				Address:  ":9090",
				File:     defaultFilePath,
				Interval: ds(defaultStoreInterval),
				Restore:  defaultRestore,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"ADDRESS", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE"} {
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
