package httpapi

import (
	"context"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMaxAgeZeroRejectsAnOlderDifferentSessionWithinSameSecond(t *testing.T) {
	ctx := context.Background()
	service, handlers, store := testAuthHTTPWithStore(t)
	user, err := service.RegisterPublic(ctx, auth.RegisterPublicRequest{Email: "fresh@example.test", Username: "fresh.user", Password: []byte("a-long-test-password")}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.CreateApplication(ctx, applications.CreateInput{ID: "fresh-app", Name: "Fresh", ClientID: "fresh-client", ApplicationType: "public", AccessPolicy: "default_allow", Enabled: true, RedirectURIs: []string{"https://example.com/callback"}, Scopes: []string{"openid"}, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	provider, err := oidc.NewService(store, &signing.Manager{}, oidc.ServiceConfig{Issuer: "https://identity.example", AuthorizationCodeTTL: time.Minute, AccessTokenTTL: time.Minute, IDTokenTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	h := newOAuthHandlers(provider, handlers, nil, nil)
	anchor := time.Now().Add(-time.Second).Truncate(time.Second)
	if err = store.CreateIdentitySession(ctx, auth.SessionRecord{ID: "older-other-session", UserID: user.ID, TokenHash: auth.HashToken("older-token"), CSRFHash: auth.HashToken("csrf"), AuthMethod: "password", Proof: auth.ProofPassword, AuthTime: anchor.Add(100 * time.Millisecond), CreatedAt: anchor, LastSeenAt: anchor, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	request := oidc.AuthorizationRequest{ResponseType: "code", ClientID: "fresh-client", RedirectURI: "https://example.com/callback", Scope: "openid", State: "state", Nonce: "nonce", CodeChallenge: oidc.S256Challenge(strings.Repeat("v", 43)), CodeChallengeMethod: "S256", MaxAge: "0"}
	handle, err := h.saveRequest(pendingAuthorization{Request: request, SessionID: "original-session", Started: anchor.Add(500 * time.Millisecond)})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/oauth/authorize?request="+handle, nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "older-token"})
	r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	rec := httptest.NewRecorder()
	h.authorize(rec, r)
	if rec.Code != 303 || !strings.Contains(rec.Header().Get("Location"), "auth=reauth") {
		t.Fatalf("older same-second session bypassed fresh auth: %d", rec.Code)
	}
}
