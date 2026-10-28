package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func TestApplicationRegistryRoundTrip(t *testing.T) {
	db, err := Open(
		filepath.Join(t.TempDir(), "identity.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := ApplyMigrations(
		ctx,
		db,
		migrations.FS,
	); err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	now := time.Now().UTC()

	err = store.CreateApplication(
		ctx,
		applications.CreateInput{
			ID:                   "app_test",
			Name:                 "MW Admin",
			ClientID:             "mw_admin",
			ClientSecretHash:     "hash-only",
			ApplicationType:      "confidential",
			AuthenticationPolicy: "standard",
			DeleteCallbackURI:    "https://app.example.com/account-deleted",
			AllowPublicSignup:    true,
			Enabled:              true,
			RedirectURIs: []string{
				"https://admin.mattediworks.com/api/auth/callback",
			},
			Scopes: []string{
				"email",
				"openid",
				"profile",
			},
			Audiences: []string{
				"mw-admin",
			},
			CreatedAt: now,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	app, err := store.ApplicationByClientID(
		ctx,
		"mw_admin",
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(app.RedirectURIs) != 1 ||
		app.RedirectURIs[0] !=
			"https://admin.mattediworks.com/api/auth/callback" {
		t.Fatalf(
			"redirect URIs = %#v",
			app.RedirectURIs,
		)
	}
	if app.ClientSecretHash != "hash-only" {
		t.Fatalf(
			"client secret hash = %q",
			app.ClientSecretHash,
		)
	}
	if app.DeleteCallbackURI != "https://app.example.com/account-deleted" || !app.AllowPublicSignup {
		t.Fatalf("lifecycle settings = callback %q, signup %v", app.DeleteCallbackURI, app.AllowPublicSignup)
	}
}

func TestDenyAssignmentRevokesExistingAccessToken(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	now := time.Now().UTC()
	if err := store.CreateApplication(ctx, applications.CreateInput{
		ID: "app_access", Name: "Access", ClientID: "access-client",
		ApplicationType: "public", AuthenticationPolicy: "standard",
		AccessPolicy: "default_allow", Enabled: true,
		RedirectURIs: []string{"https://example.com/callback"},
		Scopes:       []string{"openid"}, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO identity_users
		(id,email,email_normalized,display_name,password_hash,enabled,is_superadmin,created_at_unix,updated_at_unix)
		VALUES ('usr_access','access@example.com','access@example.com','Access','hash',1,0,?,?)`, now.Unix(), now.Unix()); err != nil {
		t.Fatal(err)
	}
	if c, e := store.IdentityConsent(ctx, "usr_access", "app_access"); e != nil {
		t.Fatal(e)
	} else if c.Version == 0 {
		if e := store.GrantIdentityConsent(ctx, "usr_access", "app_access", []string{"openid"}, 0, now); e != nil {
			t.Fatal(e)
		}
	}
	if err := store.CreateAccessToken(ctx, oidc.AccessToken{
		ConsentVersion: 1,
		ID:             "token_access", TokenHash: "token-hash", ClientID: "access-client",
		UserID: "usr_access", Scopes: []string{"openid"}, AuthTime: now,
		AuthMethod: "password", ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertIdentityAssignment(ctx, identityadmin.Assignment{
		ApplicationID: "app_access", UserID: "usr_access", Role: "user",
		AccessEffect: "deny", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AccessTokenByHash(ctx, "token-hash", now); err == nil {
		t.Fatal("denied user's existing access token is still valid")
	}
	if err := store.DeleteIdentityAssignment(ctx, "app_access", "usr_access"); err != nil {
		t.Fatal(err)
	}
	if c, e := store.IdentityConsent(ctx, "usr_access", "app_access"); e != nil {
		t.Fatal(e)
	} else if c.Version == 0 {
		if e := store.GrantIdentityConsent(ctx, "usr_access", "app_access", []string{"openid"}, 0, now); e != nil {
			t.Fatal(e)
		}
	}
	if err := store.CreateAccessToken(ctx, oidc.AccessToken{
		ConsentVersion: 1,
		ID:             "token_policy", TokenHash: "policy-token-hash", ClientID: "access-client",
		UserID: "usr_access", Scopes: []string{"openid"}, AuthTime: now,
		AuthMethod: "password", ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateIdentityApplication(ctx, "app_access", identityadmin.UpdateApplication{
		Name: "Access", AuthenticationPolicy: "standard", AccessPolicy: "default_deny",
		RedirectURIs: []string{"https://example.com/callback"}, Scopes: []string{"openid"},
	}, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AccessTokenByHash(ctx, "policy-token-hash", now); err == nil {
		t.Fatal("default-deny policy left an unassigned user's access token valid")
	}
}

func TestPurgeIdentityApplicationRequiresNoDependentDataAndFreesClientID(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	now := time.Now().UTC()
	input := applications.CreateInput{
		ID: "app_purge", Name: "Temporary", ClientID: "mw_temporary",
		ApplicationType: "public", AuthenticationPolicy: "standard",
		Enabled: true, RedirectURIs: []string{"https://example.com/callback"},
		Scopes: []string{"openid"}, CreatedAt: now,
	}
	if err := store.CreateApplication(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO identity_users
		(id,email,email_normalized,display_name,password_hash,enabled,is_superadmin,created_at_unix,updated_at_unix)
		VALUES ('usr_purge','purge@example.com','purge@example.com','Purge','hash',1,0,?,?)`, now.Unix(), now.Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO identity_application_assignments
		(application_id,user_id,role,created_at_unix,updated_at_unix)
		VALUES ('app_purge','usr_purge','user',?,?)`, now.Unix(), now.Unix()); err != nil {
		t.Fatal(err)
	}
	archivedAt := now
	if err := store.SetIdentityApplicationState(ctx, input.ID, false, &archivedAt, now); err != nil {
		t.Fatal(err)
	}

	if err := store.PurgeIdentityApplication(ctx, input.ID, input.ClientID); !errors.Is(err, identityadmin.ErrApplicationInUse) {
		t.Fatalf("purge with assignment error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM identity_application_assignments WHERE application_id = ?`, input.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.PurgeIdentityApplication(ctx, input.ID, input.ClientID); err != nil {
		t.Fatal(err)
	}

	input.ID = "app_reused"
	input.CreatedAt = now.Add(time.Second)
	if err := store.CreateApplication(ctx, input); err != nil {
		t.Fatalf("recreate freed client ID: %v", err)
	}
}

// Applications registered before the delete callback became mandatory keep
// working, are flagged, and must add a callback on their next edit. Disabling
// one stays possible.
func TestLegacyApplicationWithoutDeleteCallback(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	now := time.Now().UTC()
	for _, input := range []applications.CreateInput{
		{ID: "app_legacy", Name: "Legacy", ClientID: "legacy"},
		{ID: "app_bff", Name: "MW Admin", ClientID: "mw_admin", Internal: true},
	} {
		input.ApplicationType, input.AuthenticationPolicy, input.Enabled = "public", "standard", true
		input.RedirectURIs, input.Scopes, input.CreatedAt = []string{"https://example.com/callback"}, []string{"openid"}, now
		if err := store.CreateApplication(ctx, input); err != nil {
			t.Fatal(err)
		}
	}

	legacy, err := store.IdentityApplicationByID(ctx, "app_legacy")
	if err != nil || !legacy.NeedsDeleteCallback || legacy.Internal {
		t.Fatalf("legacy app = needs callback %v, internal %v, err %v", legacy.NeedsDeleteCallback, legacy.Internal, err)
	}
	internal, err := store.IdentityApplicationByID(ctx, "app_bff")
	if err != nil || internal.NeedsDeleteCallback || !internal.Internal {
		t.Fatalf("internal app = needs callback %v, internal %v, err %v", internal.NeedsDeleteCallback, internal.Internal, err)
	}

	admin, err := identityadmin.New(store)
	if err != nil {
		t.Fatal(err)
	}
	edit := identityadmin.UpdateApplication{
		Name: "Legacy renamed", RedirectURIs: []string{"https://example.com/callback"}, Scopes: []string{"openid"},
	}
	if _, err := admin.UpdateApplication(ctx, auth.User{ID: "usr_admin"}, "", "app_legacy", edit); !errors.Is(err, applications.ErrDeleteCallbackRequired) {
		t.Fatalf("edit without callback error = %v", err)
	}
	if _, err := admin.SetApplicationState(ctx, auth.User{ID: "usr_admin"}, "", "app_legacy", "disable"); err != nil {
		t.Fatalf("disable legacy app: %v", err)
	}

	edit.DeleteCallbackURI = "https://example.com/account-deleted"
	updated, err := admin.UpdateApplication(ctx, auth.User{ID: "usr_admin"}, "", "app_legacy", edit)
	if err != nil || updated.NeedsDeleteCallback {
		t.Fatalf("edit with callback = needs callback %v, err %v", updated.NeedsDeleteCallback, err)
	}

	edit.DeleteCallbackURI, edit.Internal = "", true
	if updated, err = admin.UpdateApplication(ctx, auth.User{ID: "usr_admin"}, "", "app_legacy", edit); err != nil || !updated.Internal {
		t.Fatalf("mark internal = internal %v, err %v", updated.Internal, err)
	}
	edit.AllowPublicSignup = true
	if _, err := admin.UpdateApplication(ctx, auth.User{ID: "usr_admin"}, "", "app_legacy", edit); !errors.Is(err, identityadmin.ErrInvalid) && !errors.Is(err, applications.ErrInvalidApplication) {
		t.Fatalf("internal app with public sign-up error = %v", err)
	}
}
