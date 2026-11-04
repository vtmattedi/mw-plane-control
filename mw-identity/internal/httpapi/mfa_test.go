package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testOTP(secret []byte, step int64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(b[:])
	sum := mac.Sum(nil)
	o := sum[19] & 15
	v := binary.BigEndian.Uint32(sum[o:o+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", v%1000000)
}
func TestCombinedMFARequiresPasswordBrowserAndOneUseChallenge(t *testing.T) {
	ctx := context.Background()
	service, handlers, store := testAuthHTTPWithStore(t)
	password := "a-long-enough-password"
	user, err := service.CreateSuperadmin(ctx, "mfa@example.test", "MFA", []byte(password))
	if err != nil {
		t.Fatal(err)
	}
	strong, err := identitystrong.NewService(store, identitystrong.Options{SecretKeys: "1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), ActiveKeyVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	setup, err := strong.SetupTOTP(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
	if err != nil {
		t.Fatal(err)
	}
	step := time.Now().Unix() / 30
	if err = strong.ConfirmTOTP(ctx, user.ID, testOTP(secret, step-1)); err != nil {
		t.Fatal(err)
	}
	h := newIdentityStrongHandlers(strong, nil, handlers, time.Minute)
	call := func(body map[string]string, browser string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/totp/login", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: loginCSRFCookieName, Value: browser})
		rec := httptest.NewRecorder()
		h.loginTOTP(rec, req)
		return rec
	}
	csrf := "browser-proof"
	if rec := call(map[string]string{"email": user.Email, "code": testOTP(secret, step), "csrf_token": csrf}, csrf); rec.Code != 401 {
		t.Fatalf("OTP-only accepted: %d", rec.Code)
	}
	rec := call(map[string]string{"email": user.Email, "password": password, "csrf_token": csrf, "return_to": "/account"}, csrf)
	if rec.Code != 200 || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("password stage issued session or failed: %d", rec.Code)
	}
	var started struct {
		Challenge string `json:"challenge"`
	}
	if err = json.Unmarshal(rec.Body.Bytes(), &started); err != nil || started.Challenge == "" {
		t.Fatal("no challenge")
	}
	finish := map[string]string{"challenge": started.Challenge, "code": testOTP(secret, step), "csrf_token": "other-browser"}
	if rec = call(finish, "other-browser"); rec.Code != 401 {
		t.Fatal("cross-browser challenge accepted")
	}
	finish["csrf_token"] = csrf
	rec = call(finish, csrf)
	if rec.Code != 200 {
		t.Fatalf("MFA failed: %d %s", rec.Code, rec.Body.String())
	}
	var token string
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			token = c.Value
		}
	}
	session, err := service.Session(ctx, token)
	if err != nil || !auth.Satisfies(session.Session.Proof, auth.ACRStrong) || auth.Satisfies(session.Session.Proof, auth.ACRWebAuthn) {
		t.Fatalf("incorrect evidence: %+v %v", session.Session, err)
	}
	if rec = call(finish, csrf); rec.Code != 401 {
		t.Fatal("replayed challenge accepted")
	}
}
