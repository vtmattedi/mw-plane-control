package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

func TestRecentBFFWebAuthn(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	fresh := session.Record{AuthMethod: "webauthn", AuthTime: now.Add(-2 * time.Minute)}
	if !recentBFFWebAuthn(fresh, 5*time.Minute, now) {
		t.Fatal("fresh WebAuthn session was not considered recent")
	}
	fallback := fresh
	fallback.AuthMethod = "totp"
	if recentBFFWebAuthn(fallback, 5*time.Minute, now) {
		t.Fatal("TOTP session satisfied WebAuthn freshness")
	}
	stale := fresh
	stale.AuthTime = now.Add(-6 * time.Minute)
	if recentBFFWebAuthn(stale, 5*time.Minute, now) {
		t.Fatal("stale WebAuthn session satisfied freshness")
	}
}

func TestSensitiveFederatedRoutes(t *testing.T) {
	if !sensitiveOAuthRoute(http.MethodPost, "/admin/api/projects/4/rotate-key") {
		t.Fatal("OAuth API-key rotation was not marked sensitive")
	}
	if !sensitiveOAuthRoute(http.MethodPost, "/admin/api/projects/4/rotate-callback-secret") {
		t.Fatal("OAuth callback-secret rotation was not marked sensitive")
	}
	if !sensitiveOAuthRoute(http.MethodPut, "/admin/api/projects/4/oauth/google") {
		t.Fatal("OAuth policy update was not marked sensitive")
	}
	if sensitiveOAuthRoute(http.MethodGet, "/admin/api/projects") {
		t.Fatal("OAuth read was incorrectly marked sensitive")
	}
	if !sensitiveMailRoute(http.MethodPost, "/admin/api/projects/8/rotate-key") {
		t.Fatal("Mail API-key rotation was not marked sensitive")
	}
	if sensitiveMailRoute(http.MethodGet, "/admin/api/projects") {
		t.Fatal("Mail read was incorrectly marked sensitive")
	}
}
