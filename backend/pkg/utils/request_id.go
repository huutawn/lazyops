package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func NewRequestID() string {
	return NewPrefixedID("req")
}

func NewCorrelationID() string {
	return NewPrefixedID("corr")
}

func NewPrefixedID(prefix string) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}

	return prefix + "_" + hex.EncodeToString(buf)
}
