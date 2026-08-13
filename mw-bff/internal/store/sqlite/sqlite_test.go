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

	var firstCount int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(1) FROM schema_migrations`,
	).Scan(&firstCount); err != nil {
		t.Fatal(err)
	}

	if firstCount == 0 {
		t.Fatal("expected at least one applied migration")
	}

	if err := ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}

	var secondCount int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(1) FROM schema_migrations`,
	).Scan(&secondCount); err != nil {
		t.Fatal(err)
	}

	if secondCount != firstCount {
		t.Fatalf(
			"migration count changed on second apply: first=%d second=%d",
			firstCount,
			secondCount,
		)
	}
}
