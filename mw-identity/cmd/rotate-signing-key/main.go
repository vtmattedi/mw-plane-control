package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(
			fmt.Sprintf(
				"configuration failed: %v",
				err,
			),
		)
	}

	keyring, err := signing.ParseKeyring(
		cfg.SigningKeys,
		cfg.SigningActiveKeyVersion,
	)
	if err != nil {
		fatal(
			fmt.Sprintf(
				"signing configuration failed: %v",
				err,
			),
		)
	}

	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		fatal(
			fmt.Sprintf(
				"database open failed: %v",
				err,
			),
		)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	if err := storesqlite.ApplyMigrations(
		ctx,
		db,
		migrations.FS,
	); err != nil {
		fatal(
			fmt.Sprintf(
				"database migration failed: %v",
				err,
			),
		)
	}

	manager, err := signing.NewManager(
		storesqlite.NewStore(db),
		keyring,
	)
	if err != nil {
		fatal(err.Error())
	}

	kid, err := manager.Rotate(ctx)
	if err != nil {
		fatal(
			fmt.Sprintf(
				"rotate signing key failed: %v",
				err,
			),
		)
	}

	fmt.Printf(
		"rotated signing key; active kid=%s\n",
		kid,
	)
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "ERROR:", message)
	os.Exit(1)
}
