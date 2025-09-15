package main

import (
	"os"
	"testing"
)

func TestFlags_applyServerFlags(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		args    []string
		check   func(t *testing.T)
		wantErr bool
	}{
		{
			name: "sets address",
			args: []string{"-a=0.0.0.0:9999"},
			check: func(t *testing.T) {
				if os.Getenv("ADDRESS") != "0.0.0.0:9999" {
					t.Errorf("ADDRESS not set")
				}
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"-z"},
			wantErr: true,
		},
		{
			name: "env override",
			env: map[string]string{
				"ADDRESS": "from-env",
			},
			args: []string{"-a=127.0.0.1:9999"},
			check: func(t *testing.T) {
				if os.Getenv("ADDRESS") != "from-env" {
					t.Errorf("ADDRESS override failed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			err := applyServerFlags(tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.check != nil {
				tt.check(t)
			}
		})
	}
}
