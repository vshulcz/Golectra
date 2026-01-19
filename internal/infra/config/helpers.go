package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// FromEnvOrFlag returns the environment value when present, otherwise falls back to a CLI flag then default.
func FromEnvOrFlag(envKey, flagVal, def string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	if v := strings.TrimSpace(flagVal); v != "" {
		return v
	}
	return def
}

// FromEnvOrFlagBool merges boolean values from ENV and flags (defaulting to def).
func FromEnvOrFlagBool(envKey string, flagVal, def bool) bool {
	if ev := strings.TrimSpace(os.Getenv(envKey)); ev != "" {
		return envBool(envKey, def)
	}
	if flagVal {
		return true
	}
	return def
}

// FromEnvOrFlagInt resolves integer values with minimum validation.
func FromEnvOrFlagInt(envKey string, flagVal, def, min int) int {
	if ev := strings.TrimSpace(os.Getenv(envKey)); ev != "" {
		if n, err := strconv.Atoi(ev); err == nil && n >= min {
			return n
		}
	}
	if flagVal != 0 && flagVal >= min {
		return flagVal
	}
	return def
}

// FromEnvOrFlagDuration reads a duration (seconds or Go syntax) with fallbacks and reports whether it came from config.
func FromEnvOrFlagDuration(envKey string, flagSeconds, flagSentinel, defSeconds int) (time.Duration, bool) {
	if ev := strings.TrimSpace(os.Getenv(envKey)); ev != "" {
		if n, err := strconv.ParseInt(ev, 10, 64); err == nil {
			return time.Duration(n) * time.Second, true
		}
		if d, err := time.ParseDuration(ev); err == nil {
			return d, true
		}
		return envDuration(envKey, time.Duration(defSeconds)*time.Second), true
	}
	if flagSeconds != flagSentinel {
		return time.Duration(flagSeconds) * time.Second, true
	}
	return time.Duration(defSeconds) * time.Second, false
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		if n <= 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(v); err == nil {
		if d <= 0 {
			return 0
		}
		return d
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "t", "yes", "y":
		return true
	case "0", "false", "f", "no", "n":
		return false
	default:
		return def
	}
}
