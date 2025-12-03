package misc

import (
	"crypto/sha256"
	"encoding/hex"
)

// SumSHA256 returns a hex-encoded SHA256 checksum of value concatenated with key.
func SumSHA256(value []byte, key string) string {
	sum := sha256.Sum256(append(value, []byte(key)...))
	return hex.EncodeToString(sum[:])
}
