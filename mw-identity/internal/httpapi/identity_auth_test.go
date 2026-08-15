package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func testAuthHTTP(t *testing.T) (*auth.Service, *identityAuthHandlers) {
	t.Helper()

	db, err := storesqlite.Open(
		filepath.Join(t.TempDir(), "identity.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := storesqlite.ApplyMigrations(
		context.Background(),
		db,
		migrations.FS,
	); err != nil {
		t.Fatal(err)
	}

	store := storesqlite.NewStore(db)
	service, err := auth.NewService(
		store,
		store,
		auth.ServiceConfig{
			SessionTTL: time.Hour,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	return service, newIdentityAuthHandlers(
		service,
		IdentityAuthConfig{
			AppEnv:                 "test",
			LoginCSRFTTL:           10 * time.Minute,
			LoginRateLimitAttempts: 10,
			LoginRateLimitWindow:   5 * time.Minute,
		},
	)
}

func TestLoginPageCookiePolicy(t *testing.T) {
	_, handlers := testAuthHTTP(t)

	req := httptest.NewRequest(
		http.MethodGet,
		"/login",
		nil,
	)
	rec := httptest.NewRecorder()

	handlers.loginPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Boas-vindas") ||
		!strings.Contains(body, "/static/brand.svg#mark") {
		t.Fatalf("new Portuguese portal was not rendered: %q", body)
	}

	var csrf *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == loginCSRFCookieName {
			csrf = cookie
			break
		}
	}

	if csrf == nil {
		t.Fatal("login CSRF cookie missing")
	}
	if csrf.HttpOnly {
		t.Fatal("login CSRF cookie should not be HttpOnly")
	}
	if csrf.SameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite = %v", csrf.SameSite)
	}
	if csrf.Secure {
		t.Fatal("test/development cookie should not be Secure")
	}
}

func TestEnglishLoginPage(t *testing.T) {
	_, handlers := testAuthHTTP(t)
	req := httptest.NewRequest(http.MethodGet, "/login/en", nil)
	rec := httptest.NewRecorder()

	handlers.loginPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Welcome") ||
		!strings.Contains(body, `lang="en"`) {
		t.Fatalf("English portal was not rendered: %q", body)
	}
}

func TestLogoutRejectsBadCSRF(t *testing.T) {
	service, handlers := testAuthHTTP(t)
	password := []byte("correct horse battery staple")

	_, err := service.CreateSuperadmin(
		context.Background(),
		"admin@example.com",
		"Admin",
		password,
	)
	if err != nil {
		t.Fatal(err)
	}

	login, err := service.Login(
		context.Background(),
		"admin@example.com",
		password,
		"127.0.0.1",
	)
	if err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader(
		"csrf_token=definitely-wrong",
	)
	req := httptest.NewRequest(
		http.MethodPost,
		"/logout",
		body,
	)
	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: login.SessionToken,
	})
	req.AddCookie(&http.Cookie{
		Name:  csrfCookieName,
		Value: login.CSRFToken,
	})

	rec := httptest.NewRecorder()
	handlers.logout(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}

	if _, err := service.Session(
		context.Background(),
		login.SessionToken,
	); err != nil {
		t.Fatalf("session should remain valid: %v", err)
	}
}
