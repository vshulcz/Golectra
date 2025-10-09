package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/internal/misc"
)

func FromEnvOrFlag(envKey, flagVal, def string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	if v := strings.TrimSpace(flagVal); v != "" {
		return v
	}
	return def
}

func FromEnvOrFlagBool(envKey string, flagVal, def bool) bool {
	if ev := strings.TrimSpace(os.Getenv(envKey)); ev != "" {
		return misc.GetBool(envKey, def)
	}
	if flagVal {
		return true
	}
	return def
}

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

func FromEnvOrFlagDuration(envKey string, flagSeconds, flagSentinel, defSeconds int) (time.Duration, bool) {
	if ev := strings.TrimSpace(os.Getenv(envKey)); ev != "" {
		if n, err := strconv.ParseInt(ev, 10, 64); err == nil {
			return time.Duration(n) * time.Second, true
		}
		if d, err := time.ParseDuration(ev); err == nil {
			return d, true
		}
		return misc.GetDuration(envKey, time.Duration(defSeconds)*time.Second), true
	}
	if flagSeconds != flagSentinel {
		return time.Duration(flagSeconds) * time.Second, true
	}
	return time.Duration(defSeconds) * time.Second, false
}
