package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/identityclient"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/secure"
)

type oauthErrorCallbackStore struct {
	allowedStateHash string
	consumed         bool
	fail            error
}

func (*oauthErrorCallbackStore) CreateOIDCTransaction(context.Context, identityclient.Transaction) error {
	return nil
}

func (s *oauthErrorCallbackStore) ClaimOIDCTransaction(_ context.Context, hash string, _ time.Time) (identityclient.Transaction, error) {
	if s.fail != nil {
		return identityclient.Transaction{}, s.fail
	}
	if s.consumed || hash != s.allowedStateHash {
		return identityclient.Transaction{}, identityclient.ErrTransactionNotFound
	}
	s.consumed = true
	return identityclient.Transaction{}, nil
}

func (*oauthErrorCallbackStore) DeleteExpiredOIDCTransactions(context.Context, time.Time) error {
	return nil
}

func newOAuthErrorCallbackHandler(t *testing.T) (*authHandlers, *oauthErrorCallbackStore) {
	t.Helper()
	sum := sha256.Sum256([]byte("valid-state"))
	store := &oauthErrorCallbackStore{allowedStateHash: hex.EncodeToString(sum[:])}
	client, err := identityclient.New(identityclient.Config{
		Issuer:         "https://identity.example.test",
		InternalBaseURL:"http://identity.internal.test",
		ClientID:       "admin",
		ClientSecret:   "test-secret",
		CallbackURL:    "https://admin.example.test/api/auth/callback",
		Scopes:         []string{"openid", "identity.admin"},
		TransactionTTL: time.Minute,
		HTTPTimeout:    time.Second,
	}, store, secure.Keyring{})
	if err != nil {
		t.Fatal(err)
	}
	return newAuthHandlers(client, nil, AuthConfig{AppEnv: "test"}), store
}

func TestIdentityAuthorizationErrorCallback(t *testing.T) {
	tests := []struct {
		name, oauthError, description, state string
		wantStatus                             int
		wantCode                               string
	}{
		{
			name: "private app policy denial", oauthError: "access_denied",
			description: "identity is denied by the application access policy",
			state: "valid-state", wantStatus: 403, wantCode: "identity_application_access_denied",
		},
		{
			name: "consent declined", oauthError: "access_denied",
			description: "consent declined",
			state: "valid-state", wantStatus: 403, wantCode: "identity_consent_declined",
		},
		{
			name: "unspecified denial", oauthError: "access_denied",
			description: "arbitrary-unsafe-marker",
			state: "valid-state", wantStatus: 403, wantCode: "identity_access_denied",
		},
		{
			name: "invalid authorization request", oauthError: "invalid_request",
			description: "requested scope is not allowed",
			state: "valid-state", wantStatus: 400, wantCode: "identity_authorization_request_rejected",
		},
		{
			name: "provider outage", oauthError: "temporarily_unavailable",
			description: "arbitrary-unsafe-marker",
			state: "valid-state", wantStatus: 502, wantCode: "identity_provider_unavailable",
		},
		{
			name: "unknown provider error", oauthError: "arbitrary-unsafe-marker",
			description: "arbitrary-unsafe-marker",
			state: "valid-state", wantStatus: 400, wantCode: "identity_authorization_failed",
		},
		{
			name: "forged error with wrong state", oauthError: "access_denied",
			description: "identity is denied by the application access policy",
			state: "wrong-state", wantStatus: 400, wantCode: "invalid_or_expired_state",
		},
		{
			name: "missing state", oauthError: "access_denied",
			state: "", wantStatus: 400, wantCode: "invalid_or_expired_state",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, store := newOAuthErrorCallbackHandler(t)
			query := url.Values{"error": {tc.oauthError}, "error_description": {tc.description}}
			if tc.state != "" {
				query.Set("state", tc.state)
			}
			rec := httptest.NewRecorder()
			handler.callback(rec, httptest.NewRequest(http.MethodGet, "/auth/callback?"+query.Encode(), nil))
			if rec.Code != tc.wantStatus {
				t.Fatalf("HTTP %d; want %d", rec.Code, tc.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), `"code":"`+tc.wantCode+`"`) {
				t.Fatalf("unexpected response: %q", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "arbitrary-unsafe-marker") {
				t.Fatal("untrusted provider error reflected to browser")
			}
			if rec.Header().Get("Cache-Control") != "no-store, max-age=0" {
				t.Fatal("OAuth failure response must not be cached")
			}
			if rec.Header().Get("Set-Cookie") != "" {
				t.Fatal("failed authorization issued a session cookie")
			}
			if store.consumed != (tc.state == "valid-state") {
				t.Fatal("failed authorization consumed an unrelated state")
			}

			if tc.state == "valid-state" {
				replay := httptest.NewRecorder()
				handler.callback(replay, httptest.NewRequest(http.MethodGet, "/auth/callback?"+query.Encode(), nil))
				if replay.Code != 400 || !strings.Contains(replay.Body.String(), "invalid_or_expired_state") {
					t.Fatal("replayed authorization error was accepted")
				}
			}
		})
	}
}
