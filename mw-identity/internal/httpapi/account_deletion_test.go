package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

type accountBrowser map[string]string

func (b accountBrowser) request(handler http.Handler, method, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, strings.SplitN(path, "#", 2)[0], strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for name, value := range b {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(b, cookie.Name)
		} else {
			b[cookie.Name] = cookie.Value
		}
	}
	return rec
}

func accountRoutes(h *identityAuthHandlers) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /account", h.account)
	mux.HandleFunc("POST /account/delete", h.deleteAccount)
	mux.HandleFunc("POST /account/applications/{id}/remove", h.removeApplication)
	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("POST /login", h.login)
	mux.HandleFunc("POST /register", h.register)
	return mux
}

func TestPasswordOnlySignupCanDeleteWholeAccount(t *testing.T) {
	for _, durable := range []bool{false, true} {
		t.Run(map[bool]string{false: "dispatch paused", true: "dispatch enabled"}[durable], func(t *testing.T) {
			service, h, store := testAuthHTTPWithStore(t)
			service.SetDeletionDispatch(durable, false)
			h.config.PublicSignupEnabled = true
			browser := accountBrowser{loginCSRFCookieName: "login-csrf-test"}
			form := standaloneForm("password.deletion")
			form.Set("csrf_token", "login-csrf-test")
			mux := accountRoutes(h)
			if rec := browser.request(mux, "POST", "/register", form); rec.Code != 303 {
				t.Fatalf("signup: %d %s", rec.Code, rec.Body.String())
			}
			session, err := service.Session(context.Background(), browser[sessionCookieName])
			if err != nil || session.Session.Proof != auth.ProofPassword || session.Session.AuthMethod != "password" {
				t.Fatalf("signup assurance: %+v %v", session, err)
			}
			page := browser.request(mux, "GET", "/account?lang=en", nil)
			if page.Code != 200 || !strings.Contains(page.Body.String(), `action="/account/delete"`) {
				t.Fatal("fresh password signup has no deletion confirmation")
			}
			csrf := browser[csrfCookieName]
			bad := browser.request(mux, "POST", "/account/delete", url.Values{"csrf_token": {"wrong"}, "confirmation": {"DELETE"}})
			if bad.Code != 403 {
				t.Fatal("invalid CSRF accepted")
			}
			rec := browser.request(mux, "POST", "/account/delete", url.Values{"csrf_token": {csrf}, "confirmation": {"DELETE"}})
			if rec.Code != 303 || strings.Contains(rec.Header().Get("Location"), "auth=strong") {
				t.Fatalf("password deletion: %d %s", rec.Code, rec.Body.String())
			}
			if _, err := store.IdentityUserByID(context.Background(), session.User.ID); err == nil {
				t.Fatal("unconnected password-only account retained")
			}
			if browser[sessionCookieName] != "" {
				t.Fatal("deletion left an authenticated session")
			}
		})
	}
}

func TestStaleSessionReauthenticatesBeforeDeletionWithoutAutomaticReplay(t *testing.T) {
	ctx := context.Background()
	service, h, store := testAuthHTTPWithStore(t)
	user, err := service.RegisterPublic(ctx, auth.RegisterPublicRequest{Email: "stale@example.test", Username: "stale.delete", Password: []byte("a-long-test-password")}, "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err = store.CreateIdentitySession(ctx, auth.SessionRecord{ID: "stale", UserID: user.ID, TokenHash: auth.HashToken("old-session"), CSRFHash: auth.HashToken("old-csrf"), Proof: auth.ProofPassword, AuthMethod: "password", AuthTime: now.Add(-time.Hour), CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	browser := accountBrowser{sessionCookieName: "old-session", csrfCookieName: "old-csrf"}
	mux := accountRoutes(h)
	page := browser.request(mux, "GET", "/account?lang=en", nil)
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Verify my identity again") || strings.Contains(page.Body.String(), `action="/account/delete"`) {
		t.Fatal("stale account page does not offer reauthentication")
	}
	staleForm := url.Values{"csrf_token": {"old-csrf"}, "confirmation": {"DELETE"}}
	blocked := browser.request(mux, "POST", "/account/delete", staleForm)
	if blocked.Code != 303 || blocked.Header().Get("Location") != accountReauthenticationURL {
		t.Fatalf("no Standard reauth path: %d %s", blocked.Code, blocked.Header().Get("Location"))
	}
	if got, err := store.IdentityUserByID(ctx, user.ID); err != nil || !got.Enabled || got.DeletionRequestedAt != nil {
		t.Fatal("stale request initiated deletion")
	}
	loginPage := browser.request(mux, "GET", accountReauthenticationURL, nil)
	if loginPage.Code != 200 || !strings.Contains(loginPage.Body.String(), `data-required-auth="reauth"`) {
		t.Fatal("existing session bypassed explicit reauthentication")
	}
	returnTo := "/account#delete-account"
	login := browser.request(mux, "POST", "/login", url.Values{"csrf_token": {browser[loginCSRFCookieName]}, "email": {user.Email}, "password": {"a-long-test-password"}, "return_to": {returnTo}})
	if login.Code != 303 || login.Header().Get("Location") != returnTo {
		t.Fatalf("password reauth: %d %s", login.Code, login.Body.String())
	}
	if got, err := store.IdentityUserByID(ctx, user.ID); err != nil || !got.Enabled {
		t.Fatal("reauthentication automatically replayed deletion")
	}
	if replay := browser.request(mux, "POST", "/account/delete", staleForm); replay.Code != 403 {
		t.Fatal("old confirmation/CSRF replayed with new session")
	}
	page = browser.request(mux, "GET", returnTo, nil)
	if !strings.Contains(page.Body.String(), `action="/account/delete"`) {
		t.Fatal("fresh password session cannot confirm deletion")
	}
	deleted := browser.request(mux, "POST", "/account/delete", url.Values{"csrf_token": {browser[csrfCookieName]}, "confirmation": {"DELETE"}})
	if deleted.Code != 303 {
		t.Fatalf("fresh confirmation: %d %s", deleted.Code, deleted.Body.String())
	}
	if _, err := store.IdentityUserByID(ctx, user.ID); err == nil {
		t.Fatal("fresh confirmed deletion did not remove account")
	}
}

func TestConsentRevocationDoesNotRequireStrongOrRecentAuthentication(t *testing.T) {
	ctx := context.Background()
	service, h, store := testAuthHTTPWithStore(t)
	h.config.StrongAuthMaxAge = time.Minute
	user, err := service.RegisterPublic(ctx, auth.RegisterPublicRequest{Email: "revoke@example.test", Username: "revoke.user", Password: []byte("a-long-test-password")}, "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := store.CreateApplication(ctx, applications.CreateInput{ID: "revoke-app", Name: "Revoke", ClientID: "revoke-app", ApplicationType: "public", AccessPolicy: "default_allow", Enabled: true, Scopes: []string{"openid"}, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.GrantIdentityConsent(ctx, user.ID, "revoke-app", []string{"openid"}, 0, now); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateIdentitySession(ctx, auth.SessionRecord{ID: "revoke", UserID: user.ID, TokenHash: auth.HashToken("old-session"), CSRFHash: auth.HashToken("csrf"), Proof: auth.ProofPassword, AuthMethod: "password", AuthTime: now.Add(-time.Hour), CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	browser := accountBrowser{sessionCookieName: "old-session", csrfCookieName: "csrf"}
	mux := accountRoutes(h)
	form := url.Values{"csrf_token": {"invalid"}, "confirm": {"yes"}}
	if rec := browser.request(mux, "POST", "/account/applications/revoke-app/remove", form); rec.Code != 403 {
		t.Fatal("revocation accepted invalid CSRF")
	}
	form.Set("csrf_token", "csrf")
	rec := browser.request(mux, "POST", "/account/applications/revoke-app/remove", form)
	if rec.Code != 303 || rec.Header().Get("Location") != "/account?notice=app_removed" {
		t.Fatalf("unnecessary reauth redirect: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	consent, err := store.IdentityConsent(ctx, user.ID, "revoke-app")
	if err != nil || consent.Active {
		t.Fatal("consent not revoked")
	}
}

func TestAccountDeletionAssuranceRejectsRecoveryAndInvalidProof(t *testing.T) {
	h := &identityAuthHandlers{}
	now := time.Now().UTC()
	for _, session := range []auth.Session{
		{AuthMethod: "recovery", AuthTime: now},
		{Proof: auth.ProofPassword, AuthMethod: "webauthn", AuthTime: now},
		{Proof: auth.ProofPassword, AuthMethod: "password", AuthTime: now.Add(time.Hour)},
		{Proof: auth.ProofPassword, AuthMethod: "password", AuthTime: now.Add(-time.Hour)},
	} {
		if h.recentAccountAuthentication(session) {
			t.Fatalf("invalid deletion assurance accepted: %+v", session)
		}
	}
	for _, proof := range []string{auth.ProofPassword, auth.ProofMFA, auth.ProofPasskey} {
		if !h.recentAccountAuthentication(auth.Session{Proof: proof, AuthMethod: auth.MethodForProof(proof), AuthTime: now}) {
			t.Fatalf("Standard-or-higher proof rejected: %s", proof)
		}
	}
}
