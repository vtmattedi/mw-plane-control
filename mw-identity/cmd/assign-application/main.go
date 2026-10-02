package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func main() {
	email := flag.String("email", "", "Identity user email")
	clientID := flag.String("client-id", "", "Identity application client_id")
	role := flag.String("role", "user", "assignment role: user, admin, or superadmin")
	accessEffect := flag.String("access", "allow", "access exception: allow or deny")
	flag.Parse()

	normalizedEmail, err := auth.NormalizeEmail(strings.TrimSpace(*email))
	if err != nil {
		fatal("valid --email is required")
	}
	if strings.TrimSpace(*clientID) == "" {
		fatal("--client-id is required")
	}

	cfg, err := config.Load()
	if err != nil {
		fatal(err.Error())
	}

	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		fatal(err.Error())
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := storesqlite.ApplyMigrations(ctx, db, migrations.FS); err != nil {
		fatal(err.Error())
	}

	store := storesqlite.NewStore(db)

	user, err := store.IdentityUserByEmail(ctx, normalizedEmail)
	if err != nil {
		fatal("Identity user not found")
	}

	app, err := store.ApplicationByClientID(ctx, strings.TrimSpace(*clientID))
	if err != nil {
		fatal("Identity application not found")
	}

	service, err := identityadmin.New(store)
	if err != nil {
		fatal(err.Error())
	}

	assignment, err := service.UpsertAssignment(
		ctx,
		user,
		"operator-cli",
		app.ID,
		user.ID,
		strings.TrimSpace(*role),
		strings.TrimSpace(*accessEffect),
	)
	if err != nil {
		fatal(err.Error())
	}

	fmt.Printf(
		"set %s access to %s as %s (%s)\n",
		user.ID,
		app.ClientID,
		assignment.Role,
		assignment.AccessEffect,
	)
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "ERROR:", message)
	os.Exit(1)
}
