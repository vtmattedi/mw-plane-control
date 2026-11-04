package sqlite

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

type removalTestSigner struct{}

func (removalTestSigner) SignJWT(context.Context, map[string]any) (string, error) {
	return "signed.lifecycle.assertion", nil
}

type removalFixture struct {
	store   *Store
	service *auth.Service
	now     time.Time
}

// newRemovalFixture creates a user connected to two applications: "app_one"
// (assignment + token, deletion callback at callbackURL) and "app_two" (token
// only). Only app_one should be affected by removing app_one.
func newRemovalFixture(t *testing.T, callbackURL string) removalFixture {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	now := time.Now().UTC()

	for _, app := range []struct{ id, clientID, callback string }{
		{"app_one", "one-client", callbackURL},
		{"app_two", "two-client", ""},
	} {
		if err := store.CreateApplication(ctx, applications.CreateInput{
			ID: app.id, Name: app.id, ClientID: app.clientID,
			ApplicationType: "public", AccessPolicy: "default_allow", DeleteCallbackURI: app.callback, Enabled: true,
			RedirectURIs: []string{"https://example.com/callback"},
			Scopes:       []string{"openid"}, CreatedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO identity_users
		(id,email,email_normalized,display_name,password_hash,enabled,is_superadmin,created_at_unix,updated_at_unix)
		VALUES ('usr_remove','remove@example.com','remove@example.com','Remove','hash',1,0,?,?)`, now.Unix(), now.Unix()); err != nil {
		t.Fatal(err)
	}
	for _, app := range []string{"app_one", "app_two"} {
		if err := store.GrantIdentityConsent(ctx, "usr_remove", app, []string{"openid"}, 0, now); err != nil {
			t.Fatal(err)
		}
	}
	for _, token := range []struct{ id, hash, clientID string }{
		{"token_one", "hash-one", "one-client"},
		{"token_two", "hash-two", "two-client"},
	} {
		if err := store.CreateAccessToken(ctx, oidc.AccessToken{
			ConsentVersion: 1, ID: token.id, TokenHash: token.hash, ClientID: token.clientID,
			UserID: "usr_remove", Scopes: []string{"openid"}, AuthTime: now,
			AuthMethod: "password", ExpiresAt: now.Add(time.Hour), CreatedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpsertIdentityAssignment(ctx, identityadmin.Assignment{
		ApplicationID: "app_one", UserID: "usr_remove", Role: "user",
		AccessEffect: "allow", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	service, err := auth.NewService(store, store, auth.ServiceConfig{SessionTTL: time.Hour, PublicIssuer: "https://identity.example"})
	if err != nil {
		t.Fatal(err)
	}
	service.SetLifecycleSigner(removalTestSigner{})
	return removalFixture{store: store, service: service, now: now}
}

func connectedIDs(t *testing.T, store *Store) []string {
	t.Helper()
	apps, err := store.ListIdentityUserApplications(context.Background(), "usr_remove")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(apps))
	for _, app := range apps {
		ids = append(ids, app.ID)
	}
	return ids
}

func TestRemoveConnectedApplicationRevokesOnlyThatApplication(t *testing.T) {
	var called bool
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer callback.Close()
	f := newRemovalFixture(t, callback.URL)
	ctx := context.Background()

	delivery, err := f.service.RemoveConnectedApplication(ctx, "usr_remove", "app_one", "127.0.0.1")
	if err != nil || delivery.Status != "disconnected" {
		t.Fatalf("RemoveConnectedApplication() = %#v, %v", delivery, err)
	}
	if called {
		t.Fatal("disconnect sent a destructive callback")
	}
	if _, err := f.store.AccessTokenByHash(ctx, "hash-one", f.now); err == nil {
		t.Fatal("removed application's token is still valid")
	}
	if _, err := f.store.AccessTokenByHash(ctx, "hash-two", f.now); err != nil {
		t.Fatalf("other application's token was revoked: %v", err)
	}
	if _, err := f.store.IdentityAssignment(ctx, "app_one", "usr_remove"); err != nil {
		t.Fatalf("assignment still present: %v", err)
	}
	if ids := connectedIDs(t, f.store); len(ids) != 1 || ids[0] != "app_two" {
		t.Fatalf("connected applications after removal = %v", ids)
	}
}

func TestDisconnectDoesNotDependOnCallbackAvailability(t *testing.T) {
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer callback.Close()
	f := newRemovalFixture(t, callback.URL)
	ctx := context.Background()

	delivery, err := f.service.RemoveConnectedApplication(ctx, "usr_remove", "app_one", "127.0.0.1")
	if err != nil || delivery.Status != "disconnected" {
		t.Fatalf("RemoveConnectedApplication() = %#v, %v", delivery, err)
	}
	if _, err := f.store.AccessTokenByHash(ctx, "hash-one", f.now); err == nil {
		t.Fatal("provider credentials must be revoked without callback delivery")
	}
	if _, err := f.store.IdentityAssignment(ctx, "app_one", "usr_remove"); err != nil {
		t.Fatalf("administrator assignment must survive disconnect: %v", err)
	}
}

func TestRemoveConnectedApplicationNeverDeletesDenyAssignment(t *testing.T) {
	f := newRemovalFixture(t, "")
	ctx := context.Background()
	if err := f.store.UpsertIdentityAssignment(ctx, identityadmin.Assignment{
		ApplicationID: "app_one", UserID: "usr_remove", Role: "user",
		AccessEffect: "deny", CreatedAt: f.now, UpdatedAt: f.now,
	}); err != nil {
		t.Fatal(err)
	}

	delivery, err := f.service.RemoveConnectedApplication(ctx, "usr_remove", "app_one", "127.0.0.1")
	if err != nil || delivery.Status != "disconnected" {
		t.Fatalf("RemoveConnectedApplication() = %#v, %v", delivery, err)
	}
	assignment, err := f.store.IdentityAssignment(ctx, "app_one", "usr_remove")
	if err != nil || assignment.AccessEffect != "deny" {
		t.Fatalf("deny assignment was removed: %#v, %v", assignment, err)
	}
}

func TestRemoveConnectedApplicationRejectsUnconnectedApplication(t *testing.T) {
	f := newRemovalFixture(t, "")
	if _, err := f.service.RemoveConnectedApplication(context.Background(), "usr_remove", "app_missing", "127.0.0.1"); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
