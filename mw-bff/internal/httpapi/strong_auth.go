package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

func recentBFFWebAuthn(record session.Record, maxAge time.Duration, now time.Time) bool {
	if strings.TrimSpace(record.AuthMethod) != "webauthn" || record.AuthTime.IsZero() {
		return false
	}
	age := now.UTC().Sub(record.AuthTime.UTC())
	return age >= 0 && age <= maxAge
}

func assuranceFor(record session.Record) string {
	switch strings.TrimSpace(record.AuthMethod) {
	case "webauthn":
		return "high"
	case "totp":
		return "fallback"
	case "password":
		return "legacy"
	case "recovery":
		return "recovery"
	default:
		return "unknown"
	}
}

func requireBFFWebAuthn(w http.ResponseWriter, record session.Record, maxAge time.Duration) bool {
	if recentBFFWebAuthn(record, maxAge, time.Now()) {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"code": "step_up_required", "step_up_url": "/api/auth/step-up"}})
	return false
}

func sensitiveOAuthRoute(method, path string) bool {
	if method == http.MethodPost && (strings.HasSuffix(path, "/rotate-key") || strings.HasSuffix(path, "/rotate-callback-secret")) {
		return true
	}
	return method == http.MethodPut && strings.HasSuffix(path, "/oauth/google")
}

func sensitiveMailRoute(method, path string) bool {
	return method == http.MethodPost && strings.HasSuffix(path, "/rotate-key")
}
