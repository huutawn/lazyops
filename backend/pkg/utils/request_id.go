package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func NewRequestID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "req-fallback"
	}

	return hex.EncodeToString(buf)
}
