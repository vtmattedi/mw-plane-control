package main

import (
	"context"
	"flag"
	"fmt"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-bff/internal/store/sqlite"
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
	if err := storesqlite.VerifyDatabase(ctx, *path, []string{"bff_admin_sessions", "bff_oidc_transactions", "bff_delegated_tokens"}); err != nil {
		fatal(err.Error())
	}
	fmt.Println("BFF database verification PASS")
}
func fatal(m string) { fmt.Fprintln(os.Stderr, "ERROR:", m); os.Exit(1) }
