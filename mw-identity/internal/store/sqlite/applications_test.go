package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
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
