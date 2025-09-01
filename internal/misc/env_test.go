package misc

import (
	"testing"
	"time"
)

func TestGetenv(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		val    string
		def    string
		expect string
	}{
		{"value present", "X_FOO", "bar", "zzz", "bar"},
		{"value empty -> default", "X_EMPTY", "", "defv", "defv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val != "" {
				t.Setenv(tt.key, tt.val)
			} else {
				t.Setenv(tt.key, "")
			}
			got := Getenv(tt.key, tt.def)
			if got != tt.expect {
				t.Errorf("Getenv(%s) = %q, want %q", tt.key, got, tt.expect)
			}
		})
	}
}

func TestGetDuration(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		val    string
		def    time.Duration
		expect time.Duration
	}{
		{"valid duration", "X_OK", "5s", 0, 5 * time.Second},
		{"bad format -> default", "X_BAD", "oops", 3 * time.Second, 3 * time.Second},
		{"empty -> default", "X_EMPTY", "", 7 * time.Second, 7 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.val != "" {
				t.Setenv(tt.key, tt.val)
			} else {
				t.Setenv(tt.key, "")
			}
			got := GetDuration(tt.key, tt.def)
			if got != tt.expect {
				t.Errorf("GetDuration(%s) = %v, want %v", tt.key, got, tt.expect)
			}
		})
	}
}
