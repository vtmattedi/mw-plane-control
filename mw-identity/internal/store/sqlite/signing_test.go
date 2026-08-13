package sqlite

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func TestSQLiteSigningRotation(t *testing.T) {
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

	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = 9
	}

	keyring, err := signing.ParseKeyring(
		"1:"+
			base64.RawURLEncoding.EncodeToString(raw),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	manager, err := signing.NewManager(
		store,
		keyring,
	)
	if err != nil {
		t.Fatal(err)
	}

	first, err := manager.EnsureActiveKey(ctx)
	if err != nil {
		t.Fatal(err)
	}

	second, err := manager.Rotate(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatal("rotation reused signing kid")
	}

	jwks, err := manager.PublicJWKS(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 2 {
		t.Fatalf(
			"JWKS key count = %d; want 2",
			len(jwks.Keys),
		)
	}
}
