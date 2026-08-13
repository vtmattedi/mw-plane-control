package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vtmattedi/mw-plane-control/mw-bff/migrations"
)

func TestApplyMigrationsIsIdempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()

	if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(1) FROM schema_migrations`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("migration count = %d; want 1", count)
	}
}
