package sqlite

import (
	"context"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/identityclient"
	"github.com/vtmattedi/mw-plane-control/mw-bff/migrations"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func TestStepUpMigrationRejectsUnboundLegacyTransactions(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "bff.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "006_step_up_binding.sql" {
			continue
		}
		raw, err := fs.ReadFile(migrations.FS, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[entry.Name()] = &fstest.MapFile{Data: raw}
	}
	if err = ApplyMigrations(ctx, db, before); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO bff_oidc_transactions(id,state_hash,nonce_hash,pkce_verifier_ciphertext,pkce_verifier_nonce,encryption_key_version,return_to,expires_at_unix,created_at_unix) VALUES ('old','old-state','nonce',X'01',X'02',1,'/',9999999999,1)`); err != nil {
		t.Fatal(err)
	}
	if err = ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	if _, err = store.ClaimOIDCTransaction(ctx, "old-state", time.Now()); err == nil {
		t.Fatal("unbound legacy transaction survived")
	}
	now := time.Now()
	tx := identityclient.Transaction{ID: "new", StateHash: "new-state", NonceHash: "nonce", PKCEVerifierCiphertext: []byte{1}, PKCEVerifierNonce: []byte{2}, EncryptionKeyVersion: 1, ReturnTo: "/", ExpiresAt: now.Add(time.Minute), CreatedAt: now, ExpectedSubject: "subject", RequiredACR: "urn:mw:webauthn"}
	if err = store.CreateOIDCTransaction(ctx, tx); err != nil {
		t.Fatal(err)
	}
	got, err := store.ClaimOIDCTransaction(ctx, tx.StateHash, now)
	if err != nil || got.ExpectedSubject != tx.ExpectedSubject || got.RequiredACR != tx.RequiredACR {
		t.Fatal("step-up binding lost in persistent transaction")
	}
}
