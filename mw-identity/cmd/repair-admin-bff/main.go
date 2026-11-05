// Command repair-admin-bff is an operator-only, narrowly scoped repair for
// existing MW Admin BFF registrations. It is intentionally unavailable over HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
)

func main() {
	clientID := flag.String("client-id", "", "existing BFF_OIDC_CLIENT_ID (required)")
	redirectURI := flag.String("redirect-uri", "", "exact registered HTTPS admin callback (required)")
	apply := flag.Bool("apply", false, "perform the single-field update; default is dry-run")
	flag.Parse()

	if flag.NArg() != 0 || strings.TrimSpace(*clientID) == "" || strings.TrimSpace(*redirectURI) == "" {
		fatal("usage: /app/repair-admin-bff --client-id ID --redirect-uri https://HOST/api/auth/callback [--apply]")
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

	// Do NOT apply migrations here: this is a surgical repair of an existing
	// deployment, not a deployment or database upgrade operation.
	result, err := storesqlite.NewStore(db).RepairAdminBFFInternal(
		ctx, *clientID, *redirectURI, *apply, time.Now().UTC(),
	)
	if err != nil {
		switch {
		case errors.Is(err, storesqlite.ErrAdminBFFRepairNotFound):
			fatal("client not found; verify BFF_OIDC_CLIENT_ID without creating a new registration")
		case errors.Is(err, storesqlite.ErrAdminBFFRepairIneligible):
			fatal("safety checks failed: " + err.Error())
		default:
			fatal(err.Error())
		}
	}

	switch {
	case result.AlreadyInternal:
		fmt.Printf("UNCHANGED: %s (%s) is already internal=true; registration checks passed\n", result.Name, result.ClientID)
	case result.Changed:
		fmt.Printf("UPDATED: %s (%s) now has internal=true; audited in the Identity database\n", result.Name, result.ClientID)
	default:
		fmt.Printf("DRY RUN: %s (%s) is eligible for internal=true; no data changed\n", result.Name, result.ClientID)
		fmt.Println("After verifying an off-container database backup, re-run with --apply.")
	}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "ERROR:", message)
	os.Exit(1)
}
