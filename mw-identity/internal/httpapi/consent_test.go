package httpapi

import (
	"context"
	"encoding/base64"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
)

func TestPublicConsentDisconnectReconsentAndRecentAuthentication(t *testing.T) {
	ctx := context.Background()
	service, authHandlers, store := testAuthHTTPWithStore(t)
	err := store.CreateApplication(ctx, applications.CreateInput{ID: "public_app", Name: "WOI", ClientID: "woi", ApplicationType: "public", AuthenticationPolicy: "standard", AccessPolicy: "default_deny", AllowPublicSignup: true, Enabled: true, RedirectURIs: []string{"https://app.example/callback"}, Scopes: []string{"openid", "profile", "email"}, CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	user, err := service.RegisterPublic(ctx, auth.RegisterPublicRequest{Email: "consent@example.test", Username: "consent.user", Password: []byte("a-long-test-password")}, "")
	if err != nil {
		t.Fatal(err)
	}
	keyring, err := signing.ParseKeyring("1:"+base64.RawURLEncoding.EncodeToString(make([]byte, 32)), 1)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signing.NewManager(store, keyring)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := signer.EnsureActiveKey(ctx); err != nil {
		t.Fatal(err)
	}
	provider, err := oidc.NewService(store, signer, oidc.ServiceConfig{Issuer: "https://identity.example", AuthorizationCodeTTL: time.Minute, AccessTokenTTL: time.Minute, IDTokenTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	h := newOAuthHandlers(provider, authHandlers, nil, nil)
	login, err := service.Login(ctx, user.Email, []byte("a-long-test-password"), "")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: login.SessionToken})
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: login.CSRFToken})
		rec := httptest.NewRecorder()
		if method == "POST" {
			h.consent(rec, req)
		} else {
			h.authorize(rec, req)
		}
		return rec
	}
	verifier := strings.Repeat("v", 43)
	query := url.Values{"response_type": {"code"}, "client_id": {"woi"}, "redirect_uri": {"https://app.example/callback"}, "scope": {"openid profile email"}, "state": {"browser-state"}, "nonce": {"browser-nonce"}, "code_challenge": {oidc.S256Challenge(verifier)}, "code_challenge_method": {"S256"}, "ui_locales": {"en"}}
	authorize := "/oauth/authorize?" + query.Encode()
	extract := func(body string) string {
		t.Helper()
		m := regexp.MustCompile(`name="request" value="([^"]+)"`).FindStringSubmatch(body)
		if len(m) != 2 {
			t.Fatalf("no consent form: %s", body)
		}
		return m[1]
	}
	consent := call("GET", authorize, nil)
	if consent.Code != 200 || !strings.Contains(consent.Body.String(), "Continue with MW") {
		t.Fatalf("consent response %d %s", consent.Code, consent.Body.String())
	}
	handle := extract(consent.Body.String())
	if rec := call("POST", "/oauth/consent", url.Values{"request": {handle}, "decision": {"allow"}}); rec.Code != 403 {
		t.Fatalf("CSRF accepted: %d", rec.Code)
	}
	consent = call("GET", authorize, nil)
	handle = extract(consent.Body.String())
	approved := call("POST", "/oauth/consent", url.Values{"request": {handle}, "csrf_token": {login.CSRFToken}, "decision": {"allow"}})
	if approved.Code != 200 {
		t.Fatalf("approval: %d %s", approved.Code, approved.Body.String())
	}
	link := regexp.MustCompile(`<a class="primary-button" href="([^"]+)"`).FindStringSubmatch(approved.Body.String())
	if len(link) != 2 {
		t.Fatal("missing CSP-safe handoff")
	}
	target, _ := url.Parse(html.UnescapeString(link[1]))
	code := target.Query().Get("code")
	if code == "" {
		t.Fatal("no code")
	}
	tokens, e := provider.ExchangeCode(ctx, oidc.TokenRequest{GrantType: "authorization_code", ClientID: "woi", Code: code, RedirectURI: "https://app.example/callback", CodeVerifier: verifier})
	if e != nil || tokens.IDToken == "" {
		t.Fatalf("real provider exchange: %+v", e)
	}
	if _, e := provider.UserInfo(ctx, tokens.AccessToken); e != nil {
		t.Fatal(e)
	}
	if _, err := service.RemoveConnectedApplication(ctx, user.ID, "public_app", ""); err != nil {
		t.Fatal(err)
	}
	if _, e := provider.UserInfo(ctx, tokens.AccessToken); e == nil {
		t.Fatal("old token survived disconnect")
	}
	consent = call("GET", authorize, nil)
	handle = extract(consent.Body.String())
	if _, err := service.RemoveConnectedApplication(ctx, user.ID, "public_app", ""); err != nil {
		t.Fatal(err)
	}
	// An explicit consent form for the already-revoked version is valid; a form
	// from before an active connection was revoked is covered by store fencing.
	approved = call("POST", "/oauth/consent", url.Values{"request": {handle}, "csrf_token": {login.CSRFToken}, "decision": {"allow"}})
	if approved.Code != 200 {
		t.Fatalf("reconsent: %d %s", approved.Code, approved.Body.String())
	}
	query.Set("max_age", "0")
	rec := call("GET", "/oauth/authorize?"+query.Encode(), nil)
	if rec.Code != 303 {
		t.Fatalf("max_age=0 did not reauthenticate: %d", rec.Code)
	}
	loginURL, _ := url.Parse(rec.Header().Get("Location"))
	resume := loginURL.Query().Get("return_to")
	same := call("GET", resume, nil)
	if same.Code != 303 {
		t.Fatalf("same session bypassed reauthentication: %d", same.Code)
	}
	login, err = service.Login(ctx, user.Email, []byte("a-long-test-password"), "")
	if err != nil {
		t.Fatal(err)
	}
	recent := call("GET", resume, nil)
	if recent.Code != 302 {
		t.Fatalf("fresh session loop: %d %s", recent.Code, recent.Body.String())
	}
	// Cross-site entry can omit a Strict session cookie. Bind max_age=0 to
	// an opaque request before the FIRST login, rather than demanding two logins.
	missing := httptest.NewRecorder()
	h.authorize(missing, httptest.NewRequest("GET", "/oauth/authorize?"+query.Encode(), nil))
	if missing.Code != 303 {
		t.Fatalf("unauthenticated recent-auth flow: %d", missing.Code)
	}
	firstLogin, _ := url.Parse(missing.Header().Get("Location"))
	firstResume := firstLogin.Query().Get("return_to")
	parsedResume, _ := url.Parse(firstResume)
	if parsedResume.Query().Get("request") == "" {
		t.Fatal("unauthenticated max_age=0 lost its fresh-authentication marker")
	}
	login, err = service.Login(ctx, user.Email, []byte("a-long-test-password"), "")
	if err != nil {
		t.Fatal(err)
	}
	if rec := call("GET", firstResume, nil); rec.Code != 302 {
		t.Fatalf("first login did not complete recent authentication: %d", rec.Code)
	}
	apps, err := store.ListIdentityUserDeletionApplications(ctx, user.ID)
	if err != nil || len(apps) != 1 {
		t.Fatalf("history: %v %v", apps, err)
	}
}
