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
)

func registrationRequest(form url.Values) *http.Request {
	form.Set("csrf_token", "login-csrf-test")
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: loginCSRFCookieName, Value: "login-csrf-test"})
	return req
}

func standaloneForm(username string) url.Values {
	return url.Values{
		"username": {username},
		"email":    {username + "@example.com"},
		"password": {"a-long-enough-password"},
		"language": {"en"},
	}
}

func TestStandaloneRegisterPageFollowsSwitch(t *testing.T) {
	_, handlers := testAuthHTTP(t)

	rec := httptest.NewRecorder()
	handlers.registerPage(rec, httptest.NewRequest(http.MethodGet, "/register", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("disabled stand-alone sign-up status = %d", rec.Code)
	}

	handlers.config.PublicSignupEnabled = true
	for path, want := range map[string]string{
		"/register":    "ecossistema",
		"/register/en": "One MattediWorks account",
	} {
		rec := httptest.NewRecorder()
		handlers.registerPage(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("%s status = %d, missing %q", path, rec.Code, want)
		}
		if !strings.Contains(rec.Body.String(), `name="website"`) {
			t.Fatalf("%s has no honeypot field", path)
		}
	}
}

func TestLoginOffersStandaloneSignupOnlyOnPlainVisit(t *testing.T) {
	_, handlers := testAuthHTTP(t)
	handlers.config.PublicSignupEnabled = true

	for _, test := range []struct {
		path string
		want string
	}{
		{"/login", `href="/register"`},
		{"/login/en", `href="/register/en"`},
		{"/login?return_to=%2Faccount", ""},
		{"/login?auth=strong", ""},
	} {
		rec := httptest.NewRecorder()
		handlers.loginPage(rec, httptest.NewRequest(http.MethodGet, test.path, nil))
		body := rec.Body.String()
		if test.want != "" && !strings.Contains(body, test.want) {
			t.Fatalf("%s does not offer sign-up (%s)", test.path, test.want)
		}
		if test.want == "" && strings.Contains(body, `href="/register`) {
			t.Fatalf("%s offers stand-alone sign-up", test.path)
		}
	}

	handlers.config.PublicSignupEnabled = false
	rec := httptest.NewRecorder()
	handlers.loginPage(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if strings.Contains(rec.Body.String(), `href="/register`) {
		t.Fatal("disabled sign-up is still offered on the login page")
	}
}

func TestStandaloneRegistrationCreatesUnconnectedPublicAccount(t *testing.T) {
	service, handlers := testAuthHTTP(t)
	handlers.config.PublicSignupEnabled = true

	rec := httptest.NewRecorder()
	handlers.register(rec, registrationRequest(standaloneForm("standalone.user")))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account" {
		t.Fatalf("status = %d, location = %q, body = %q", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}

	var sessionToken string
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			sessionToken = cookie.Value
		}
	}
	session, err := service.Session(context.Background(), sessionToken)
	if err != nil {
		t.Fatalf("new account has no session: %v", err)
	}
	if session.User.AccountType != "public" || session.User.IsSuperadmin {
		t.Fatalf("account type = %q, superadmin = %v", session.User.AccountType, session.User.IsSuperadmin)
	}
	apps, err := service.ConnectedApplications(context.Background(), session.User.ID)
	if err != nil || len(apps) != 0 {
		t.Fatalf("stand-alone account has connected applications: %v, %v", apps, err)
	}
}

func TestStandaloneRegistrationRejectedWhenDisabled(t *testing.T) {
	_, handlers := testAuthHTTP(t)

	rec := httptest.NewRecorder()
	handlers.register(rec, registrationRequest(standaloneForm("blocked.user")))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRegistrationHoneypotRejectsBots(t *testing.T) {
	_, handlers := testAuthHTTP(t)
	handlers.config.PublicSignupEnabled = true

	form := standaloneForm("bot.user")
	form.Set("website", "https://spam.example")
	rec := httptest.NewRecorder()
	handlers.register(rec, registrationRequest(form))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			t.Fatal("honeypot submission received a session")
		}
	}
}

func TestLoginPathFollowsRequestedLanguage(t *testing.T) {
	for _, test := range []struct {
		query, acceptLanguage, want string
	}{
		{"", "", "/login"},
		{"ui_locales=en", "", "/login/en"},
		{"ui_locales=en-GB", "pt-BR", "/login/en"},
		{"ui_locales=pt-BR+en", "en-US", "/login"},
		{"", "en-US,en;q=0.9", "/login/en"},
		{"", "pt-BR,pt;q=0.9,en;q=0.8", "/login"},
		{"", "english", "/login"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+test.query, nil)
		if test.acceptLanguage != "" {
			req.Header.Set("Accept-Language", test.acceptLanguage)
		}
		if got := loginPath(req); got != test.want {
			t.Fatalf("query %q, Accept-Language %q: loginPath = %q, want %q", test.query, test.acceptLanguage, got, test.want)
		}
	}
}

// John's path: an app with public sign-up sends him to Identity, where the
// login offers sign-up in his language, also when the app requires strong auth.
func TestAppSignupLinkMatchesLanguageAndStrongMode(t *testing.T) {
	_, handlers, store := testAuthHTTPWithStore(t)
	if err := store.CreateApplication(context.Background(), applications.CreateInput{
		ID: "app_woi", Name: "Working On It", ClientID: "woi-test",
		ApplicationType: "confidential", AuthenticationPolicy: "standard",
		AllowPublicSignup: true, Enabled: true,
		RedirectURIs: []string{"https://woi.example/callback"},
		Scopes:       []string{"openid"}, CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	returnTo := url.QueryEscape("/oauth/authorize?client_id=woi-test&response_type=code")

	for _, test := range []struct {
		path, want string
	}{
		{"/login?return_to=" + returnTo, `href="/register?client_id=woi-test`},
		{"/login/en?return_to=" + returnTo, `href="/register/en?client_id=woi-test`},
		{"/login/en?auth=strong&return_to=" + returnTo, `class="step-up-signup"`},
	} {
		rec := httptest.NewRecorder()
		handlers.loginPage(rec, httptest.NewRequest(http.MethodGet, test.path, nil))
		if !strings.Contains(rec.Body.String(), test.want) {
			t.Fatalf("%s does not contain %q", test.path, test.want)
		}
	}

	rec := httptest.NewRecorder()
	handlers.loginPage(rec, httptest.NewRequest(http.MethodGet, "/login/en?auth=strong", nil))
	if strings.Contains(rec.Body.String(), `class="step-up-signup"`) {
		t.Fatal("a step-up without an application offers sign-up")
	}
}

// A form POST may not redirect off-origin under the CSP, so a sign-up that
// continues to an application hands off through /continue.
func TestAppSignupHandsOffThroughContinue(t *testing.T) {
	_, handlers, store := testAuthHTTPWithStore(t)
	if err := store.CreateApplication(context.Background(), applications.CreateInput{
		ID: "app_woi", Name: "Working On It", ClientID: "woi-test",
		ApplicationType: "public", AuthenticationPolicy: "standard",
		AllowPublicSignup: true, Enabled: true,
		RedirectURIs: []string{"https://woi.example/callback"},
		Scopes:       []string{"openid"}, CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	authorize := "/oauth/authorize?client_id=woi-test&response_type=code"
	form := standaloneForm("john")
	form.Set("client_id", "woi-test")
	form.Set("return_to", authorize)

	rec := httptest.NewRecorder()
	handlers.register(rec, registrationRequest(form))
	want := "/continue?to=" + url.QueryEscape(authorize) + "&lang=en"
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
		t.Fatalf("status = %d, location = %q, want %q", rec.Code, rec.Header().Get("Location"), want)
	}
}

func TestContinuePageOnlyLeadsWithinIdentity(t *testing.T) {
	_, handlers := testAuthHTTP(t)
	for to, want := range map[string]string{
		"/oauth/authorize?client_id=woi": `url=/oauth/authorize?client_id=woi"`,
		"//evil.example/steal":           `url=/account"`,
		"https://evil.example":           `url=/account"`,
	} {
		rec := httptest.NewRecorder()
		handlers.continuePage(rec, httptest.NewRequest(http.MethodGet, "/continue?to="+url.QueryEscape(to), nil))
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("to=%q: page does not contain %q", to, want)
		}
	}
}

func TestStrongSetupContinueOnlyResumesAuthorization(t *testing.T) {
	authorize := url.QueryEscape("/oauth/authorize?client_id=woi")
	for query, want := range map[string]string{
		"":                         "",
		"continue_to=" + authorize: "",
		"strong_setup_required=1&continue_to=" + authorize:       "/oauth/authorize?client_id=woi",
		"strong_setup_required=1&continue_to=%2Faccount":         "",
		"strong_setup_required=1&continue_to=%2F%2Fevil.example": "",
	} {
		req := httptest.NewRequest(http.MethodGet, "/account?"+query, nil)
		if got := strongSetupContinue(req); got != want {
			t.Fatalf("%q: strongSetupContinue = %q, want %q", query, got, want)
		}
	}
}

func TestRegistrationHasItsOwnRateLimit(t *testing.T) {
	_, handlers := testAuthHTTP(t)
	handlers.config.PublicSignupEnabled = true

	for i := 0; i < registrationRateAttempts; i++ {
		rec := httptest.NewRecorder()
		// An invalid username fails fast but still spends an attempt.
		handlers.register(rec, registrationRequest(standaloneForm("!")))
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("limited after %d attempts", i)
		}
	}
	rec := httptest.NewRecorder()
	handlers.register(rec, registrationRequest(standaloneForm("late.user")))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("status = %d after the registration budget", rec.Code)
	}
	if !handlers.limiter.Allow("192.0.2.1") {
		t.Fatal("registration attempts consumed the login budget")
	}
}
