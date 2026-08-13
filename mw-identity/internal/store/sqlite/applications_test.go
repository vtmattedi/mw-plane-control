package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
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
}
