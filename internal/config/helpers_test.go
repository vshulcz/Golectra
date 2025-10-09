package config

import (
	"testing"
	"time"
)

func TestHelpers_FromEnvOrFlag(t *testing.T) {
	const key = "CFG_STR"
	tests := []struct {
		name   string
		env    string
		flag   string
		def    string
		expect string
	}{
		{
			name:   "env takes precedence over flag",
			env:    "  env-val  ",
			flag:   "flag-val",
			def:    "def",
			expect: "env-val",
		},
		{
			name:   "flag used when env empty",
			env:    "",
			flag:   "  flag-val  ",
			def:    "def",
			expect: "flag-val",
		},
		{
			name:   "default used when both empty",
			env:    "   ",
			flag:   "   ",
			def:    "def",
			expect: "def",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.env)
			got := FromEnvOrFlag(key, tc.flag, tc.def)
			if got != tc.expect {
				t.Fatalf("got %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestHelpers_FromEnvOrFlagBool(t *testing.T) {
	const key = "CFG_BOOL"
	tests := []struct {
		name   string
		env    string
		flag   bool
		def    bool
		expect bool
	}{
		{
			name:   "env true (various truthy) wins over flag false",
			env:    "TrUe",
			flag:   false,
			def:    false,
			expect: true,
		},
		{
			name:   "env false wins over flag true",
			env:    "0",
			flag:   true,
			def:    true,
			expect: false,
		},
		{
			name:   "env invalid -> falls back to def (but env has precedence path)",
			env:    "maybe",
			flag:   false,
			def:    true,
			expect: true,
		},
		{
			name:   "no env -> flag true used",
			env:    "",
			flag:   true,
			def:    false,
			expect: true,
		},
		{
			name:   "no env -> flag false -> default",
			env:    "",
			flag:   false,
			def:    true,
			expect: true,
		},
		{
			name:   "no env -> flag false -> default false",
			env:    "",
			flag:   false,
			def:    false,
			expect: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.env)
			got := FromEnvOrFlagBool(key, tc.flag, tc.def)
			if got != tc.expect {
				t.Fatalf("got %v, want %v", got, tc.expect)
			}
		})
	}
}

func TestHelpers_FromEnvOrFlagInt(t *testing.T) {
	const key = "CFG_INT"
	tests := []struct {
		name   string
		env    string
		flag   int
		def    int
		min    int
		expect int
	}{
		{
			name:   "env >= min wins",
			env:    "7",
			flag:   3,
			def:    5,
			min:    1,
			expect: 7,
		},
		{
			name:   "env < min ignored, flag >= min used",
			env:    "0",
			flag:   4,
			def:    5,
			min:    1,
			expect: 4,
		},
		{
			name:   "env invalid ignored, flag 0 ignored -> default",
			env:    "abc",
			flag:   0,
			def:    5,
			min:    1,
			expect: 5,
		},
		{
			name:   "flag < min ignored -> default",
			env:    "",
			flag:   1,
			def:    9,
			min:    2,
			expect: 9,
		},
		{
			name:   "trims env before parse",
			env:    "   12  ",
			flag:   2,
			def:    1,
			min:    1,
			expect: 12,
		},
		{
			name:   "default can be below min (function doesn't clamp def)",
			env:    "",
			flag:   0,
			def:    0,
			min:    2,
			expect: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.env)
			got := FromEnvOrFlagInt(key, tc.flag, tc.def, tc.min)
			if got != tc.expect {
				t.Fatalf("got %d, want %d", got, tc.expect)
			}
		})
	}
}

func TestHelpers_FromEnvOrFlagDuration(t *testing.T) {
	const key = "CFG_DUR"
	tests := []struct {
		name         string
		env          string
		flagSeconds  int
		sentinel     int
		defSeconds   int
		expectDur    time.Duration
		expectCustom bool
	}{
		{
			name:         "env numeric seconds wins over flag/sentinel",
			env:          "15",
			flagSeconds:  42,
			sentinel:     0,
			defSeconds:   300,
			expectDur:    15 * time.Second,
			expectCustom: true,
		},
		{
			name:         "env duration string wins",
			env:          "1m30s",
			flagSeconds:  5,
			sentinel:     0,
			defSeconds:   300,
			expectDur:    90 * time.Second,
			expectCustom: true,
		},
		{
			name:         "env invalid -> fallback via misc.GetDuration -> def",
			env:          "not-a-duration",
			flagSeconds:  10,
			sentinel:     0,
			defSeconds:   300,
			expectDur:    300 * time.Second,
			expectCustom: true,
		},
		{
			name:         "env with spaces numeric -> trimmed and used",
			env:          "   7   ",
			flagSeconds:  0,
			sentinel:     0,
			defSeconds:   300,
			expectDur:    7 * time.Second,
			expectCustom: true,
		},
		{
			name:         "flag used when env empty (sentinel=0, flag>0)",
			env:          "",
			flagSeconds:  10,
			sentinel:     0,
			defSeconds:   300,
			expectDur:    10 * time.Second,
			expectCustom: true,
		},
		{
			name:         "flag==sentinel -> default used, custom=false",
			env:          "",
			flagSeconds:  0,
			sentinel:     0,
			defSeconds:   300,
			expectDur:    300 * time.Second,
			expectCustom: false,
		},
		{
			name:         "server-style: sentinel=-1, flag=0 (sync) is valid and used",
			env:          "",
			flagSeconds:  0,
			sentinel:     -1,
			defSeconds:   300,
			expectDur:    0 * time.Second,
			expectCustom: true,
		},
		{
			name:         "server-style: sentinel=-1, flag=-1 -> default",
			env:          "",
			flagSeconds:  -1,
			sentinel:     -1,
			defSeconds:   300,
			expectDur:    300 * time.Second,
			expectCustom: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.env)
			got, custom := FromEnvOrFlagDuration(key, tc.flagSeconds, tc.sentinel, tc.defSeconds)
			if got != tc.expectDur || custom != tc.expectCustom {
				t.Fatalf("got (%v, %v), want (%v, %v)", got, custom, tc.expectDur, tc.expectCustom)
			}
		})
	}
}
