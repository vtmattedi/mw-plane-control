package main

import (
	"context"
	"flag"
	"fmt"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"os"
	"time"
)

func main() {
	path := flag.String("database", "", "database path")
	flag.Parse()
	if *path == "" {
		fatal("--database is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := storesqlite.VerifyDatabase(ctx, *path, []string{"identity_users", "identity_sessions", "identity_signing_keys", "identity_applications", "identity_deletion_operations", "identity_deletion_deliveries", "identity_mfa_challenges"}); err != nil {
		fatal(err.Error())
	}
	fmt.Println("identity database verification PASS")
}
func fatal(m string) { fmt.Fprintln(os.Stderr, "ERROR:", m); os.Exit(1) }
