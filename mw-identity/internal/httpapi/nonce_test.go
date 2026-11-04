package httpapi

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
)

func TestAuthorizationRejectsMissingNonceBeforeAuthentication(t *testing.T) {
	_, _, store := testAuthHTTPWithStore(t)
	ctx := context.Background()
	if err := store.CreateApplication(ctx, applications.CreateInput{ID: "nonce-app", Name: "Nonce", ClientID: "nonce-client", ApplicationType: "public", AccessPolicy: "default_allow", Enabled: true, RedirectURIs: []string{"https://app.example/callback"}, Scopes: []string{"openid"}, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	provider, err := oidc.NewService(store, &signing.Manager{}, oidc.ServiceConfig{Issuer: "https://identity.example", AuthorizationCodeTTL: time.Minute, AccessTokenTTL: time.Minute, IDTokenTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	// No auth handler is installed: every invalid nonce must fail before login.
	h := newOAuthHandlers(provider, nil, nil, nil)
	for _, nonce := range []string{"", "   ", strings.Repeat("x", 1025)} {
		query := url.Values{"response_type": {"code"}, "client_id": {"nonce-client"}, "redirect_uri": {"https://app.example/callback"}, "scope": {"openid"}, "state": {"request-bound-state"}, "code_challenge_method": {"S256"}, "code_challenge": {oidc.S256Challenge(strings.Repeat("v", 43))}, "nonce": {nonce}}
		rec := httptest.NewRecorder()
		h.authorize(rec, httptest.NewRequest("GET", "/oauth/authorize?"+query.Encode(), nil))
		target, err := url.Parse(rec.Header().Get("Location"))
		if err != nil || rec.Code != 302 || target.Query().Get("error") != "invalid_request" || target.Query().Get("state") != "request-bound-state" {
			t.Fatalf("invalid nonce did not fail explicitly: %d %s", rec.Code, rec.Header().Get("Location"))
		}
	}
}
