package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
)

func testIdentityAdminAPI(t *testing.T) (*storesqlite.Store, *oidc.Service, *http.ServeMux) {
	t.Helper()
	ctx := context.Background()
	_, _, store := testAuthHTTPWithStore(t)
	now := time.Now().UTC()
	for _, id := range []string{"operator", "target", "staff"} {
		if err := store.CreateIdentityUser(ctx, auth.NewUser{ID: id, Email: id + "@example.test", EmailNormalized: id + "@example.test", DisplayName: id, PasswordHash: "unused", Enabled: true, IsSuperadmin: id != "staff", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"ordinary", "administration"} {
		app := applications.CreateInput{ID: id, Name: id, ClientID: id, ApplicationType: "public", AccessPolicy: "default_allow", Enabled: true, RedirectURIs: []string{"https://app.example/callback"}, Scopes: []string{"openid"}, CreatedAt: now}
		if id == "administration" {
			app.Internal = true
			app.Audiences = []string{oidc.IdentityAdminAudience}
			app.Scopes = append(app.Scopes, "identity.admin")
		}
		if err := store.CreateApplication(ctx, app); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.GrantIdentityConsent(ctx, "operator", "administration", []string{"identity.admin"}, 0, now); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAccessToken(ctx, oidc.AccessToken{ID: "admin-token", TokenHash: auth.HashToken("admin-token"), ClientID: "administration", UserID: "operator", Audience: oidc.IdentityAdminAudience, Scopes: []string{"identity.admin"}, ConsentVersion: 1, Proof: auth.ProofPasskey, AuthMethod: "webauthn", AuthTime: now, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	ring, err := signing.ParseKeyring("1:"+base64.RawURLEncoding.EncodeToString(make([]byte, 32)), 1)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signing.NewManager(store, ring)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := oidc.NewService(store, signer, oidc.ServiceConfig{Issuer: "https://identity.example", AuthorizationCodeTTL: time.Minute, AccessTokenTTL: time.Minute, IDTokenTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	admin, err := identityadmin.New(store)
	if err != nil {
		t.Fatal(err)
	}
	h := newIdentityAdminHandlers(provider, admin, nil, nil, 5*time.Minute)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /admin/api/applications/{id}/assignments/{user_id}", h.upsertAssignment)
	mux.HandleFunc("POST /admin/api/applications", h.createApplication)
	mux.HandleFunc("PATCH /admin/api/applications/{id}", h.updateApplication)
	return store, provider, mux
}

func TestAssignmentAPIDeniesSuperadminOnlyOnOrdinaryApplications(t *testing.T) {
	ctx := context.Background()
	store, provider, mux := testIdentityAdminAPI(t)
	target, err := store.IdentityUserByID(ctx, "target")
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := store.ApplicationByClientID(ctx, "ordinary")
	if err != nil {
		t.Fatal(err)
	}
	authorization := oidc.ValidatedAuthorization{Application: ordinary, Scopes: []string{"openid"}}
	if denied := provider.AuthorizeIdentityUser(ctx, authorization, target); denied != nil {
		t.Fatalf("unassigned superadmin cannot access Public application: %v", denied)
	}
	call := func(application, userID, role, effect string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/admin/api/applications/"+application+"/assignments/"+userID, strings.NewReader(`{"role":"`+role+`","access_effect":"`+effect+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	for _, role := range []string{"user", "admin", "superadmin"} {
		t.Run(role, func(t *testing.T) {
			if rec := call("ordinary", "target", role, "allow"); rec.Code != http.StatusOK {
				t.Fatalf("allow: %d %s", rec.Code, rec.Body.String())
			}
			rec := call("ordinary", "target", role, "deny")
			if rec.Code != http.StatusOK {
				t.Fatalf("ordinary denial: %d %s", rec.Code, rec.Body.String())
			}
			var result identityadmin.Assignment
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.AccessEffect != "deny" {
				t.Fatalf("denial response: %+v %v", result, err)
			}
			if denied := provider.AuthorizeIdentityUser(ctx, authorization, target); denied == nil || denied.Code != "access_denied" {
				t.Fatalf("API denial did not prevent federation: %v", denied)
			}
			if rec := call("administration", "target", role, "deny"); rec.Code != http.StatusForbidden {
				t.Fatalf("administration denial accepted: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
	if _, err := store.IdentityAssignment(ctx, "administration", "target"); err != identityadmin.ErrNotFound {
		t.Fatalf("protected assignment changed: %v", err)
	}
	if rec := call("administration", "staff", "admin", "deny"); rec.Code != http.StatusForbidden {
		t.Fatalf("MW administrator role denial accepted: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call("administration", "staff", "admin", "allow"); rec.Code != http.StatusOK {
		t.Fatalf("MW administrator assignment failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call("administration", "staff", "user", "deny"); rec.Code != http.StatusForbidden {
		t.Fatalf("existing MW administrator could be denied by changing role: %d %s", rec.Code, rec.Body.String())
	}
	assignment, err := store.IdentityAssignment(ctx, "administration", "staff")
	if err != nil || assignment.Role != "admin" || assignment.AccessEffect != "allow" {
		t.Fatalf("protected existing assignment changed: %+v %v", assignment, err)
	}
}

func TestRegistryAPIRejectsRetiredControlsAndOmitsThemFromResponses(t *testing.T) {
	store, _, mux := testIdentityAdminAPI(t)
	base := map[string]any{"name": "Clean contract", "client_id": "clean-client", "application_type": "public", "access_policy": "default_allow", "redirect_uris": []string{"https://app.example/callback"}, "delete_callback_uri": "https://app.example/delete", "scopes": []string{"openid"}}
	call := func(method, path string, body map[string]any) *httptest.ResponseRecorder {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, strings.NewReader(string(encoded)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	for key, value := range map[string]any{"authentication_policy": "strong", "allow_public_signup": true, "access_policy_v1": false} {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		body[key] = value
		if rec := call("POST", "/admin/api/applications", body); rec.Code != 400 {
			t.Fatalf("retired creation field accepted: %s, %d", key, rec.Code)
		}
	}
	if _, err := store.ApplicationByClientID(context.Background(), "clean-client"); err == nil {
		t.Fatal("rejected creation mutated registry")
	}
	rec := call("POST", "/admin/api/applications", base)
	if rec.Code != 201 {
		t.Fatalf("clean creation: %d %s", rec.Code, rec.Body.String())
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	var application map[string]any
	if err := json.Unmarshal(response["application"], &application); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"authentication_policy", "allow_public_signup", "access_policy_v1"} {
		if _, exists := application[key]; exists {
			t.Fatalf("retired response field retained: %s", key)
		}
		body := map[string]any{}
		for k, v := range base {
			if k != "client_id" && k != "application_type" {
				body[k] = v
			}
		}
		body[key] = "obsolete"
		if rec := call("PATCH", "/admin/api/applications/"+application["id"].(string), body); rec.Code != 400 {
			t.Fatalf("retired update field accepted: %s, %d", key, rec.Code)
		}
	}
	if _, exists := response["client_secret"]; exists {
		t.Fatal("public client returned a secret")
	}
}
