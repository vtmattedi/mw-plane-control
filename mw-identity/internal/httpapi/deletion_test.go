package httpapi

import (
	"context"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDeletionProgressSurvivesSessionRevocationWithoutURLSecret(t *testing.T) {
	service, h, _ := testAuthHTTPWithStore(t)
	ctx := context.Background()
	service.SetDeletionDispatch(true, false)
	user, err := service.RegisterPublic(ctx, auth.RegisterPublicRequest{Email: "delete@example.test", Username: "delete.user", Password: []byte("a-long-test-password")}, "")
	if err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(ctx, user.Email, []byte("a-long-test-password"), "")
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"csrf_token": {login.CSRFToken}, "confirmation": {"DELETE"}}
	req := httptest.NewRequest("POST", "/account/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: login.SessionToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: login.CSRFToken})
	rec := httptest.NewRecorder()
	h.deleteAccount(rec, req)
	if rec.Code != 303 || rec.Header().Get("Location") != "/account/deletion" {
		t.Fatalf("unsafe/missing progress redirect: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	var capability *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == deletionStatusCookie {
			capability = cookie
		}
	}
	if capability == nil || !capability.HttpOnly || capability.Path != "/account/deletion" || capability.SameSite != http.SameSiteStrictMode {
		t.Fatal("status cookie not scoped")
	}
	if _, err = service.Session(ctx, login.SessionToken); err == nil {
		t.Fatal("normal session survived deletion")
	}
	statusReq := httptest.NewRequest("GET", "/account/deletion?lang=en", nil)
	statusReq.AddCookie(capability)
	status := httptest.NewRecorder()
	h.deletionProgress(status, statusReq)
	if status.Code != 200 || !strings.Contains(status.Body.String(), "deletion completed") || strings.Contains(status.Body.String(), user.ID) || strings.Contains(status.Body.String(), capability.Value) || status.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("public progress leaked identity/proof or failed")
	}
	without := httptest.NewRecorder()
	h.deletionProgress(without, httptest.NewRequest("GET", "/account/deletion?id=guessed", nil))
	if without.Code != 401 {
		t.Fatal("status allowed without capability")
	}
}
