package misc

import (
	"crypto/sha256"
	"encoding/hex"
)

func SumSHA256(value []byte, key string) string {
	sum := sha256.Sum256(append(value, []byte(key)...))
	return hex.EncodeToString(sum[:])
}
