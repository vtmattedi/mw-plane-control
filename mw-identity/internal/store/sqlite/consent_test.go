package sqlite

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func TestConsentMigrationPreservesAssignmentsWithoutInventingConsent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	before := fstest.MapFS{}
	err = fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && path < "013_application_consent.sql" {
			body, e := fs.ReadFile(migrations.FS, path)
			if e != nil {
				return e
			}
			before[path] = &fstest.MapFile{Data: body}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := ApplyMigrations(ctx, db, before); err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO identity_users (id,email,email_normalized,display_name,password_hash,created_at_unix,updated_at_unix) VALUES ('old_user','old@example.test','old@example.test','Old','hash',1,1);
 INSERT INTO identity_applications (id,name,client_id,application_type,created_at_unix,updated_at_unix) VALUES ('old_app','Old','old-client','public',1,1);
 INSERT INTO identity_application_assignments (application_id,user_id,role,access_effect,created_at_unix,updated_at_unix) VALUES ('old_app','old_user','admin','allow',1,1);
 INSERT INTO identity_applications (id,name,client_id,application_type,internal,access_policy,created_at_unix,updated_at_unix) VALUES ('internal_app','Internal','internal-client','confidential',1,'default_allow',1,1);
 INSERT INTO identity_access_tokens (id,token_hash,client_id,user_id,scope_json,expires_at_unix,created_at_unix) VALUES ('internal-token','internal-hash','internal-client','old_user','["openid"]',2000000000,1);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	c, err := store.IdentityConsent(ctx, "old_user", "old_app")
	if err != nil || c.Active || c.Version != 1 {
		t.Fatalf("invented historical consent: %+v %v", c, err)
	}
	a, err := store.IdentityAssignment(ctx, "old_app", "old_user")
	if err != nil || a.Role != "admin" || a.AccessEffect != "allow" {
		t.Fatalf("changed administrator assignment: %+v %v", a, err)
	}
	apps, err := store.ListIdentityUserDeletionApplications(ctx, "old_user")
	if err != nil || len(apps) != 2 {
		t.Fatalf("lost history: %v %v", apps, err)
	}
	if _, err := store.AccessTokenByHash(ctx, "internal-hash", time.Unix(2, 0)); err == nil {
		t.Fatal("pre-contract credential survived cutover")
	}
}

func TestConsentFencesInFlightCredentialsAndPreservesHistory(t *testing.T) {
	f := newRemovalFixture(t, "")
	ctx := context.Background()
	code := oidc.AuthorizationCode{ID: "inflight", CodeHash: "inflight", ClientID: "one-client", UserID: "usr_remove", ConsentVersion: 1, Scopes: []string{"openid"}, RedirectURI: "https://example.com/callback", CodeChallenge: "challenge", AuthTime: f.now, AuthMethod: "password", ExpiresAt: f.now.Add(time.Hour), CreatedAt: f.now}
	if err := f.store.CreateAuthorizationCode(ctx, code); err != nil {
		t.Fatal(err)
	}
	// Exchange has already read and consumed its code when disconnect happens.
	if err := f.store.ConsumeAuthorizationCode(ctx, code.CodeHash, f.now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RevokeIdentityUserApplicationAccess(ctx, "usr_remove", "one-client"); err != nil {
		t.Fatal(err)
	}
	token := oidc.AccessToken{ID: "late", TokenHash: "late", ClientID: "one-client", UserID: "usr_remove", ConsentVersion: 1, Scopes: []string{"openid"}, AuthTime: f.now, AuthMethod: "password", CreatedAt: f.now, ExpiresAt: f.now.Add(time.Hour)}
	if err := f.store.CreateAccessToken(ctx, token); err == nil {
		t.Fatal("in-flight exchange issued a credential after disconnect")
	}
	if err := f.store.CreateAuthorizationCode(ctx, code); err == nil {
		t.Fatal("stale authorization issued a code")
	}
	if err := f.store.GrantIdentityConsent(ctx, "usr_remove", "app_one", []string{"openid"}, 1, f.now); err == nil {
		t.Fatal("stale consent form undid disconnect")
	}
	c, err := f.store.IdentityConsent(ctx, "usr_remove", "app_one")
	if err != nil || c.Active || c.Version != 2 {
		t.Fatalf("revoked consent: %+v %v", c, err)
	}
	if err := f.store.GrantIdentityConsent(ctx, "usr_remove", "app_one", []string{"openid"}, c.Version, f.now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.CreateAccessToken(ctx, token); err == nil {
		t.Fatal("re-consent revived an old version")
	}
	token.ConsentVersion = 3
	if err := f.store.CreateAccessToken(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AccessTokenByHash(ctx, token.TokenHash, f.now); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RevokeIdentityUserApplicationAccess(ctx, "usr_remove", "one-client"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AccessTokenByHash(ctx, token.TokenHash, f.now); err == nil {
		t.Fatal("revoked token still validates")
	}
	apps, err := f.store.ListIdentityUserDeletionApplications(ctx, "usr_remove")
	if err != nil || len(apps) != 2 {
		t.Fatalf("lost disconnected deletion target: %v %v", apps, err)
	}
	if err := f.store.DeleteExpiredOAuthArtifacts(ctx, f.now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if ids := connectedIDs(t, f.store); len(ids) != 1 || ids[0] != "app_two" {
		t.Fatalf("token expiry changed consent: %v", ids)
	}
}

func TestPrivateApplicationIgnoresRetiredSignupMetadata(t *testing.T) {
	f := newRemovalFixture(t, "")
	ctx := context.Background()
	if _, err := f.store.db.ExecContext(ctx, `DELETE FROM identity_application_assignments; UPDATE identity_applications SET access_policy='default_deny',allow_public_signup=1 WHERE id='app_one'`); err != nil {
		t.Fatal(err)
	}
	s, err := oidc.NewService(f.store, &signing.Manager{}, oidc.ServiceConfig{Issuer: "https://identity.example", AuthorizationCodeTTL: time.Minute, AccessTokenTTL: time.Minute, IDTokenTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	user, err := f.store.IdentityUserByID(ctx, "usr_remove")
	if err != nil {
		t.Fatal(err)
	}
	app, err := f.store.ApplicationByClientID(ctx, "one-client")
	if err != nil {
		t.Fatal(err)
	}
	a := oidc.ValidatedAuthorization{Application: app, Scopes: []string{"openid"}}
	if e := s.AuthorizeIdentityUser(ctx, a, user); e == nil {
		t.Fatal("retired signup metadata bypassed private access")
	}

}
