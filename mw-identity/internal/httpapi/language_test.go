package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	webstatic "github.com/vtmattedi/mw-plane-control/mw-identity/web/static"
)

func TestPageLanguageOrder(t *testing.T) {
	for _, test := range []struct {
		name, target, cookie, accept, want string
	}{
		{"default", "/account", "", "", webstatic.LangPT},
		{"browser", "/account", "", "en-US,en;q=0.9", webstatic.LangEN},
		{"cookie beats browser", "/account", "pt-BR", "en-US", webstatic.LangPT},
		{"explicit beats cookie", "/account?lang=en", "pt-BR", "", webstatic.LangEN},
		{"en path", "/login/en", "pt-BR", "", webstatic.LangEN},
		{"unknown tag ignored", "/account?lang=fr", "", "en", webstatic.LangEN},
	} {
		req := httptest.NewRequest(http.MethodGet, test.target, nil)
		if test.cookie != "" {
			req.AddCookie(&http.Cookie{Name: languageCookieName, Value: test.cookie})
		}
		if test.accept != "" {
			req.Header.Set("Accept-Language", test.accept)
		}
		if got := pageLanguage(req); got != test.want {
			t.Fatalf("%s: pageLanguage = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestOnlyExplicitChoicesAreRemembered(t *testing.T) {
	for target, want := range map[string]string{
		"/account?lang=en": webstatic.LangEN,
		"/login/en":        webstatic.LangEN,
		"/account?lang=pt": webstatic.LangPT,
		"/account":         "",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Accept-Language", "en")
		rememberLanguage(rec, req, false)
		got := ""
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == languageCookieName {
				got = cookie.Value
				if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
					t.Fatalf("%s: language cookie attributes are wrong", target)
				}
			}
		}
		if got != want {
			t.Fatalf("%s: remembered %q, want %q", target, got, want)
		}
	}
}

func TestSignedInLanguageSwitchIsRemembered(t *testing.T) {
	service, handlers := testAuthHTTP(t)
	if _, err := service.CreateSuperadmin(context.Background(), "admin@example.com", "Admin", []byte("a-long-enough-password")); err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(context.Background(), "admin@example.com", []byte("a-long-enough-password"), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/login?lang=pt", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: login.SessionToken})
	req.AddCookie(&http.Cookie{Name: languageCookieName, Value: webstatic.LangEN})
	rec := httptest.NewRecorder()
	handlers.loginPage(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("signed-in login visit status = %d", rec.Code)
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == languageCookieName && cookie.Value == webstatic.LangPT {
			return
		}
	}
	t.Fatal("the Portuguese choice was not remembered before the redirect")
}

func TestLanguageSwitchURLKeepsTheAppContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/login/en?return_to=%2Foauth%2Fauthorize%3Fclient_id%3Dwoi", nil)
	got := languageSwitchURL(req, webstatic.LangEN)
	if !strings.HasPrefix(got, "/login?") || !strings.Contains(got, "lang=pt") ||
		!strings.Contains(got, "return_to=%2Foauth%2Fauthorize%3Fclient_id%3Dwoi") {
		t.Fatalf("languageSwitchURL = %q", got)
	}
}

func TestInfoPagesFollowTheLanguage(t *testing.T) {
	_, handlers := testAuthHTTP(t)
	for _, test := range []struct {
		target, cookie, want string
	}{
		{"/privacy", "", "Dados da conta"},
		{"/privacy", "en", "Account data"},
		{"/security?lang=en", "", "There is no “forgot my password” email."},
	} {
		req := httptest.NewRequest(http.MethodGet, test.target, nil)
		if test.cookie != "" {
			req.AddCookie(&http.Cookie{Name: languageCookieName, Value: test.cookie})
		}
		rec := httptest.NewRecorder()
		if strings.HasPrefix(test.target, "/security") {
			handlers.security(rec, req)
		} else {
			handlers.privacy(rec, req)
		}
		if !strings.Contains(rec.Body.String(), test.want) {
			t.Fatalf("%s (cookie %q) does not contain %q", test.target, test.cookie, test.want)
		}
	}
}
