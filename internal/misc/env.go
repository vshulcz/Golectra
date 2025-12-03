package misc

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Getenv returns the environment value for key or the default when empty.
func Getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// GetDuration parses an environment variable as duration in seconds or Go syntax.
func GetDuration(key string, def time.Duration) time.Duration {
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

// GetBool parses common boolean values (true/false, yes/no, 1/0) from the environment.
func GetBool(key string, def bool) bool {
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
