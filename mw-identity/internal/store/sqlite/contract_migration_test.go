package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func TestContractMigrationPreservesAccountsAndRecoveryWhileInvalidatingGrants(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "020_federation_v1_contract.sql" {
			continue
		}
		data, err := fs.ReadFile(migrations.FS, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[entry.Name()] = &fstest.MapFile{Data: data}
	}
	if err := ApplyMigrations(ctx, db, before); err != nil {
		t.Fatal(err)
	}
	s := NewStore(db)
	now := time.Now().UTC()
	for _, id := range []string{"retained", "deleting", "orphaned-request"} {
		if err := s.CreateIdentityUser(ctx, auth.NewUser{ID: id, Email: id + "@example.test", EmailNormalized: id + "@example.test", DisplayName: id, PasswordHash: "important-password-hash", Enabled: true, CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateApplication(ctx, applications.CreateInput{ID: "app", Name: "Retained", ClientID: "client", ClientSecretHash: "important-client-secret-hash", ApplicationType: "confidential", AccessPolicy: "default_deny", Enabled: true, DeleteCallbackURI: "https://app.example/delete", AccountManagementURI: "https://app.example/account", RedirectURIs: []string{"https://app.example/callback"}, Scopes: []string{"openid"}, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE identity_applications SET authentication_policy='strong',allow_public_signup=1,access_policy_v1=0`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertIdentityAssignment(ctx, identityadmin.Assignment{ApplicationID: "app", UserID: "retained", Role: "admin", AccessEffect: "allow", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"retained", "deleting"} {
		if err := s.GrantIdentityConsent(ctx, id, "app", []string{"openid"}, 0, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.BeginDurableDeletion(ctx, "deleting", now, "original-progress-hash", now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE identity_users SET enabled=0,deletion_requested_at_unix=123456 WHERE id='orphaned-request'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO identity_signing_keys(kid,alg,status,public_jwk_json,private_key_ciphertext,private_key_nonce,encryption_key_version,created_at_unix) VALUES ('preserved-key','RS256','active','{}',X'0102',X'0304',1,1); INSERT INTO identity_totp_credentials(user_id,secret_ciphertext,secret_nonce,secret_key_version,enabled,created_at_unix,updated_at_unix) VALUES ('retained','opaque-credential','opaque-nonce',1,1,1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO identity_authorization_codes(id,code_hash,client_id,user_id,redirect_uri,scope_json,code_challenge,auth_time_unix,auth_method,created_at_unix,expires_at_unix) VALUES ('code','code-hash','client','retained','https://app.example/callback','["openid"]','challenge',1,'password',1,2000000000); INSERT INTO identity_access_tokens(id,token_hash,client_id,user_id,scope_json,created_at_unix,expires_at_unix) VALUES ('token','token-hash','client','retained','["openid"]',1,2000000000)`); err != nil {
		t.Fatal(err)
	}
	tables := []string{"identity_users", "identity_applications", "identity_application_assignments", "identity_application_consents", "identity_application_redirect_uris", "identity_application_scopes", "identity_deletion_operations WHERE subject='deleting'", "identity_deletion_deliveries", "identity_signing_keys", "identity_totp_credentials"}
	snapshots := map[string]string{}
	for _, table := range tables {
		snapshots[table] = contractSnapshot(t, db, table)
	}
	// A cleanup fault must roll back the audit, grant deletion and migration marker together.
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER fail_contract_cleanup BEFORE DELETE ON identity_access_tokens BEGIN SELECT RAISE(ABORT,'simulated failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(ctx, db, migrations.FS); err == nil {
		t.Fatal("migration ignored cleanup failure")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_authorization_codes`).Scan(&count); err != nil || count != 1 {
		t.Fatal("migration partially removed grants")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_audit_events WHERE event_type='identity.federation.contract_migrated'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("migration partially committed its audit")
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER fail_contract_cleanup`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range tables {
		if !reflect.DeepEqual(snapshots[table], contractSnapshot(t, db, table)) {
			t.Fatalf("important persisted data changed: %s", table)
		}
	}
	for _, table := range []string{"identity_authorization_codes", "identity_access_tokens"} {
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("pre-contract grants retained: %s", table)
		}
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_audit_events WHERE event_type='identity.federation.contract_migrated'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("contract migration audit missing or repeated")
	}
	var importedID, status string
	var requested int64
	if err := db.QueryRowContext(ctx, `SELECT id,status,requested_at_unix FROM identity_deletion_operations WHERE subject='orphaned-request'`).Scan(&importedID, &status, &requested); err != nil || status != "legacy_review_required" || requested != 123456 {
		t.Fatal("preexisting orphaned deletion lost its recovery state")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_deletion_deliveries WHERE operation_id=?`, importedID).Scan(&count); err != nil || count != 0 {
		t.Fatal("migration fabricated replayable callback events")
	}
}

func contractSnapshot(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	rows, err := db.Query(`SELECT * FROM ` + table + ` ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var records [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		records = append(records, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
