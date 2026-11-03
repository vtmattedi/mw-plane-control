package sqlite

import (
	"context"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	"testing"
	"time"
)

func TestFederationAccessFencesCredentialsAndSuperadmins(t *testing.T) {
	f := newRemovalFixture(t, "")
	ctx := context.Background()
	if _, err := f.store.db.ExecContext(ctx, `DELETE FROM identity_application_assignments; UPDATE identity_users SET is_superadmin=1 WHERE id='usr_remove'; UPDATE identity_applications SET allow_public_signup=1 WHERE id='app_one'`); err != nil {
		t.Fatal(err)
	}
	if err := f.store.CutoverFederationAccess(ctx, "one-client", "private"); err != nil {
		t.Fatal(err)
	}
	provider, err := oidc.NewService(f.store, &signing.Manager{}, oidc.ServiceConfig{Issuer: "https://identity.example", AuthorizationCodeTTL: time.Minute, AccessTokenTTL: time.Minute, IDTokenTTL: time.Minute})
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
	if !app.AccessPolicyV1 {
		t.Fatal("cutover not persisted")
	}
	if e := provider.AuthorizeIdentityUser(ctx, oidc.ValidatedAuthorization{Application: app}, user); e == nil {
		t.Fatal("superadmin/signup bypassed private access")
	}
	token := oidc.AccessToken{ID: "late", TokenHash: "late", UserID: user.ID, ClientID: app.ClientID, ConsentVersion: 1, CreatedAt: f.now, ExpiresAt: f.now.Add(time.Hour)}
	if err := f.store.CreateAccessToken(ctx, token); err == nil {
		t.Fatal("SQL admitted unassigned superadmin")
	}
	if err := f.store.CutoverFederationAccess(ctx, "one-client", "public"); err != nil {
		t.Fatal(err)
	}
	if err := f.store.CreateAccessToken(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.db.ExecContext(ctx, `INSERT INTO identity_application_assignments(application_id,user_id,role,access_effect,created_at_unix,updated_at_unix) VALUES ('app_one','usr_remove','user','deny',1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AccessTokenByHash(ctx, "late", f.now); err == nil {
		t.Fatal("explicit deny did not fence token")
	}
	app, err = f.store.ApplicationByClientID(ctx, "one-client")
	if err != nil {
		t.Fatal(err)
	}
	if e := provider.AuthorizeIdentityUser(ctx, oidc.ValidatedAuthorization{Application: app}, user); e == nil {
		t.Fatal("Go admitted explicitly denied superadmin")
	}
}
