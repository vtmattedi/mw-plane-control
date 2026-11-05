package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

const repairRedirect = "https://admin.mattediworks.com/api/auth/callback"

func repairTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := ApplyMigrations(context.Background(), db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return db
}

func repairTestApp() applications.CreateInput {
	return applications.CreateInput{
		ID: "app_bff", Name: "MattediWorks Admin",
		ClientID: "mw-admin-bff", ClientSecretHash: "hashed-confidential-secret",
		ApplicationType: "confidential", AccessPolicy: "default_deny",
		Enabled: true, Internal: false,
		RedirectURIs: []string{repairRedirect},
		Scopes: []string{"openid", "identity.admin"},
		Audiences: []string{"mw-identity-admin"},
		CreatedAt: time.Unix(1000, 0).UTC(),
	}
}

func repairCreate(t *testing.T, db *sql.DB, input applications.CreateInput) {
	t.Helper()
	if err := NewStore(db).CreateApplication(context.Background(), input); err != nil {
		t.Fatal(err)
	}
}

func repairState(t *testing.T, db *sql.DB) (internal int, events int, secret string) {
	t.Helper()
	if err := db.QueryRow(`SELECT internal, COALESCE(client_secret_hash, '')
		FROM identity_applications WHERE client_id = 'mw-admin-bff'`).Scan(&internal, &secret); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM identity_audit_events
		WHERE event_type = 'identity.infrastructure.admin_bff_repaired'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	return
}

func TestRepairAdminBFFInternalDryRunApplyAndIdempotence(t *testing.T) {
	db := repairTestDB(t)
	input := repairTestApp()
	repairCreate(t, db, input)
	ctx := context.Background()
	now := time.Now().UTC()
	store := NewStore(db)

	dry, err := store.RepairAdminBFFInternal(ctx, input.ClientID, repairRedirect, false, now)
	if err != nil || dry.Changed || dry.AlreadyInternal {
		t.Fatalf("dry-run result = %+v, %v", dry, err)
	}
	if internal, events, secret := repairState(t, db); internal != 0 || events != 0 || secret != input.ClientSecretHash {
		t.Fatalf("dry-run mutated data: internal=%d events=%d secret=%q", internal, events, secret)
	}

	applied, err := store.RepairAdminBFFInternal(ctx, input.ClientID, repairRedirect, true, now)
	if err != nil || !applied.Changed || applied.AlreadyInternal {
		t.Fatalf("applied result = %+v, %v", applied, err)
	}
	if internal, events, secret := repairState(t, db); internal != 1 || events != 1 || secret != input.ClientSecretHash {
		t.Fatalf("unexpected mutation: internal=%d events=%d secret=%q", internal, events, secret)
	}
	var policy string
	if err := db.QueryRow("SELECT access_policy FROM identity_applications WHERE client_id = ?", input.ClientID).Scan(&policy); err != nil || policy != "default_deny" {
		t.Fatalf("access policy changed: %q, %v", policy, err)
	}

	again, err := store.RepairAdminBFFInternal(ctx, input.ClientID, repairRedirect, true, now)
	if err != nil || again.Changed || !again.AlreadyInternal {
		t.Fatalf("idempotent result = %+v, %v", again, err)
	}
	if internal, events, _ := repairState(t, db); internal != 1 || events != 1 {
		t.Fatalf("idempotence failure: internal=%d events=%d", internal, events)
	}
}

func TestRepairAdminBFFInternalRejectsUnsafeTargets(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*applications.CreateInput)
		after  func(*testing.T, *sql.DB)
		client string
		uri    string
	}{
		{"public client", func(a *applications.CreateInput) { a.ApplicationType = "public" }, nil, "", ""},
		{"no secret", func(a *applications.CreateInput) { a.ClientSecretHash = "" }, nil, "", ""},
		{"default allow", func(a *applications.CreateInput) { a.AccessPolicy = "default_allow" }, nil, "", ""},
		{"disabled", func(a *applications.CreateInput) { a.Enabled = false }, nil, "", ""},
		{"missing admin scope", func(a *applications.CreateInput) { a.Scopes = []string{"openid"} }, nil, "", ""},
		{"missing admin audience", func(a *applications.CreateInput) { a.Audiences = nil }, nil, "", ""},
		{"unregistered callback", nil, nil, "", "https://other.example.com/api/auth/callback"},
		{"wrong callback path", nil, nil, "", "https://admin.mattediworks.com/evil"},
		{"insecure callback", nil, nil, "", "http://admin.mattediworks.com/api/auth/callback"},
		{"query callback", nil, nil, "", repairRedirect + "?next=bad"},
		{"unknown client", nil, nil, "other-client", ""},
		{"archived", nil, func(t *testing.T, db *sql.DB) {
			t.Helper()
			if _, err := db.Exec("UPDATE identity_applications SET archived_at_unix = 1001 WHERE client_id = ?", "mw-admin-bff"); err != nil {
				t.Fatal(err)
			}
		}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := repairTestDB(t)
			a := repairTestApp()
			if tt.mutate != nil {
				tt.mutate(&a)
			}
			repairCreate(t, db, a)
			if tt.after != nil {
				tt.after(t, db)
			}
			clientID, redirect := a.ClientID, repairRedirect
			if tt.client != "" {
				clientID = tt.client
			}
			if tt.uri != "" {
				redirect = tt.uri
			}
			_, err := NewStore(db).RepairAdminBFFInternal(context.Background(), clientID, redirect, true, time.Now())
			if err == nil {
				t.Fatal("unsafe target was promoted")
			}
			if !errors.Is(err, ErrAdminBFFRepairIneligible) && !errors.Is(err, ErrAdminBFFRepairNotFound) {
				t.Fatalf("unexpected error: %v", err)
			}
			if internal, events, secret := repairState(t, db); internal != 0 || events != 0 || secret != a.ClientSecretHash {
				t.Fatalf("rejected repair changed data: internal=%d events=%d secret=%q", internal, events, secret)
			}
		})
	}
}

func TestRepairAdminBFFInternalRollsBackWhenAuditFails(t *testing.T) {
	db := repairTestDB(t)
	a := repairTestApp()
	repairCreate(t, db, a)
	_, err := db.Exec(`CREATE TRIGGER block_admin_bff_audit
		BEFORE INSERT ON identity_audit_events
		WHEN NEW.event_type = 'identity.infrastructure.admin_bff_repaired'
		BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewStore(db).RepairAdminBFFInternal(context.Background(), a.ClientID, repairRedirect, true, time.Now())
	if err == nil {
		t.Fatal("expected audit failure")
	}
	if internal, events, secret := repairState(t, db); internal != 0 || events != 0 || secret != a.ClientSecretHash {
		t.Fatalf("audit failure failed to roll back: internal=%d events=%d secret=%q", internal, events, secret)
	}
}
