package main

import (
	"context"
	"fmt"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/config"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-bff/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-bff/migrations"
	"os"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(err.Error())
	}
	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		fatal(err.Error())
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := storesqlite.ApplyMigrations(ctx, db, migrations.FS); err != nil {
		fatal(err.Error())
	}
	s, err := storesqlite.RunMaintenance(ctx, db, time.Now())
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("oidc_transactions=%d admin_sessions=%d delegated_tokens=%d\n", s.OIDCTransactions, s.AdminSessions, s.DelegatedTokens)
}
func fatal(m string) { fmt.Fprintln(os.Stderr, "ERROR:", m); os.Exit(1) }
