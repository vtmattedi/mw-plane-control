package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func main() {
	email := flag.String("email", "", "Identity user email")
	cancel := flag.Bool("cancel", false, "cancel pending recovery")
	flag.Parse()
	if *email == "" {
		fatal("--email is required")
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
	ctx, cancelCtx := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelCtx()
	if err := storesqlite.ApplyMigrations(ctx, db, migrations.FS); err != nil {
		fatal(err.Error())
	}
	service, err := identitystrong.NewService(storesqlite.NewStore(db), identitystrong.Options{SecretKeys: cfg.TOTPSecretKeys, ActiveKeyVersion: cfg.TOTPSecretActiveKeyVersion, Issuer: cfg.TOTPIssuer, RecoveryTTL: cfg.RecoveryTTL})
	if err != nil {
		fatal(err.Error())
	}
	if *cancel {
		if err := service.CancelRecovery(ctx, *email); err != nil {
			fatal(err.Error())
		}
		fmt.Println("Recovery cancelled; previously revoked sessions remain revoked.")
		return
	}
	result, err := service.StartRecovery(ctx, *email)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println("Recovery capability (shown once):")
	fmt.Println(result.Token)
	fmt.Println()
	fmt.Println("Recovery URL:")
	fmt.Printf("%s/recover#token=%s\n", cfg.PublicIssuer, result.Token)
	fmt.Printf("Expires: %s\n", result.ExpiresAt.Format(time.RFC3339))
}
func fatal(message string) { fmt.Fprintln(os.Stderr, "ERROR:", message); os.Exit(1) }
