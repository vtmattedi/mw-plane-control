package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func main() {
	auditDays := flag.Int("audit-retention-days", envInt("IDENTITY_AUDIT_RETENTION_DAYS", 180), "audit retention in days")
	retiredHours := flag.Int("retired-signing-retention-hours", envInt("IDENTITY_SIGNING_RETIRED_RETENTION_HOURS", 72), "retired signing-key retention in hours")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		fatal(err.Error())
	}
	minimum := int(cfg.AccessTokenTTL/time.Hour) + 1
	if *auditDays <= 0 {
		fatal("audit retention must be positive")
	}
	if *retiredHours < minimum {
		fatal(fmt.Sprintf("retired signing-key retention must be >= %d hours", minimum))
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
	stats, err := storesqlite.RunMaintenance(ctx, db, time.Now(), time.Duration(*auditDays)*24*time.Hour, time.Duration(*retiredHours)*time.Hour)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("audit=%d sessions=%d codes=%d access_tokens=%d recovery=%d retired_signing_keys=%d\n", stats.AuditEvents, stats.Sessions, stats.AuthorizationCodes, stats.AccessTokens, stats.RecoveryRequests, stats.RetiredSigningKeys)
}
func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		fatal(name + " must be a positive integer")
	}
	return value
}
func fatal(m string) { fmt.Fprintln(os.Stderr, "ERROR:", m); os.Exit(1) }
