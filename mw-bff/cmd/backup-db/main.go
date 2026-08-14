package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/config"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-bff/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-bff/migrations"
)

func main() {
	output := flag.String("output", "", "backup output path")
	flag.Parse()
	if *output == "" {
		fatal("--output is required")
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := storesqlite.ApplyMigrations(ctx, db, migrations.FS); err != nil {
		fatal(err.Error())
	}
	if err := storesqlite.BackupDatabase(ctx, db, *output); err != nil {
		fatal(err.Error())
	}
	f, err := os.Open(*output)
	if err != nil {
		fatal(err.Error())
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("backup=%s sha256=%s\n", *output, hex.EncodeToString(h.Sum(nil)))
}
func fatal(message string) { fmt.Fprintln(os.Stderr, "ERROR:", message); os.Exit(1) }
