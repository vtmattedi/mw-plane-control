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
	if method == http.MethodDelete && strings.HasPrefix(path, "/admin/api/projects/") {
		return true
	}
	if method == http.MethodPost && (strings.HasSuffix(path, "/rotate-key") || strings.HasSuffix(path, "/rotate-callback-secret")) {
		return true
	}
	return method == http.MethodPut && strings.HasSuffix(path, "/oauth/google")
}

// sensitiveMQTTRoute lists the broker operations that need fresh WebAuthn: issuing a user
// (its generated password is returned), rotating a password, re-enabling a user, deleting
// one, and everything that changes who may do what: creating or deleting roles, editing
// their rules, and giving or taking roles from clients. Disabling is deliberately not
// step-up: it only removes access, so it must stay available as a fast kill switch.
func sensitiveMQTTRoute(method, path string) bool {
	switch method {
	case http.MethodDelete:
		// Clients, roles, and a client's role assignments.
		return strings.HasPrefix(path, "/admin/api/clients/") ||
			strings.HasPrefix(path, "/admin/api/roles/")
	case http.MethodPut:
		return strings.HasPrefix(path, "/admin/api/clients/") && strings.Contains(path, "/roles/")
	case http.MethodPost:
		return path == "/admin/api/clients" ||
			path == "/admin/api/roles" ||
			strings.HasPrefix(path, "/admin/api/roles/") ||
			strings.HasSuffix(path, "/password") ||
			strings.HasSuffix(path, "/enable")
	}
	return false
}

func sensitiveMailRoute(method, path string) bool {
	if method == http.MethodDelete && strings.HasPrefix(path, "/admin/api/projects/") {
		return true
	}
	return method == http.MethodPost && strings.HasSuffix(path, "/rotate-key")
}
