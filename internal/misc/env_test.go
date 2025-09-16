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
		val    string
		def    time.Duration
		expect time.Duration
	}{
		{"valid duration", "5s", 0, 5 * time.Second},
		{"valid ms duration", "250ms", 0, 250 * time.Millisecond},
		{"negative duration", "-5s", 5 * time.Second, 0},

		{"numeric string", "10", 0, 10 * time.Second},
		{"zero numeric", "0", 5 * time.Second, 0},
		{"negative numeric", "-3", 5 * time.Second, 0},

		{"bad format -> default", "oops", 3 * time.Second, 3 * time.Second},
		{"empty -> default", "", 7 * time.Second, 7 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("X_DUR", tt.val)
			got := GetDuration("X_DUR", tt.def)
			if got != tt.expect {
				t.Errorf("val=%q def=%v -> got=%v, want=%v", tt.val, tt.def, got, tt.expect)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	trueVals := []string{"1", "true", "t", "yes", "y"}
	falseVals := []string{"0", "false", "f", "no", "n"}

	for _, v := range trueVals {
		t.Run("true_"+v, func(t *testing.T) {
			t.Setenv("X_BOOL", v)
			if !GetBool("X_BOOL", false) {
				t.Errorf("GetBool(%q) = false, want true", v)
			}
		})
	}

	for _, v := range falseVals {
		t.Run("false_"+v, func(t *testing.T) {
			t.Setenv("X_BOOL", v)
			if GetBool("X_BOOL", true) {
				t.Errorf("GetBool(%q) = true, want false", v)
			}
		})
	}

	t.Run("empty -> default true", func(t *testing.T) {
		t.Setenv("X_BOOL", "")
		if !GetBool("X_BOOL", true) {
			t.Error("expected default true")
		}
	})

	t.Run("empty -> default false", func(t *testing.T) {
		t.Setenv("X_BOOL", "")
		if GetBool("X_BOOL", false) {
			t.Error("expected default false")
		}
	})

	t.Run("unknown string -> default", func(t *testing.T) {
		t.Setenv("X_BOOL", "maybe")
		if !GetBool("X_BOOL", true) {
			t.Error("expected fallback to default true")
		}
	})
}
