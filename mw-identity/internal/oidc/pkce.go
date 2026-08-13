package oidc

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
)

var (
	pkceVerifierPattern = regexp.MustCompile(
		`^[A-Za-z0-9\-._~]{43,128}$`,
	)
	pkceChallengePattern = regexp.MustCompile(
		`^[A-Za-z0-9_-]{43}$`,
	)
)

func ValidCodeVerifier(value string) bool {
	return pkceVerifierPattern.MatchString(value)
}

func ValidS256Challenge(value string) bool {
	return pkceChallengePattern.MatchString(value)
}

func S256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
