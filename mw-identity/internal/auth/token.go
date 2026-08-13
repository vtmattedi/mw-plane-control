package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

func NewOpaqueToken(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}

	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func TokenMatchesHash(raw, expectedHash string) bool {
	actual := HashToken(raw)
	if len(actual) != len(expectedHash) {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(actual),
		[]byte(expectedHash),
	) == 1
}

func TokensEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(left),
		[]byte(right),
	) == 1
}
