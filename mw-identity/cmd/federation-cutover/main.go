// federation-cutover is a host-operator migration command, never a browser API.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
	"os"
	"strings"
	"time"
)

func main() {
	client := flag.String("client-id", "", "reviewed registered client")
	access := flag.String("access", "", "explicit intended public or private access")
	reviewed := flag.Bool("reviewed", false, "confirm backup, owner review and explicit private assignments")
	retireStrong := flag.Bool("retire-app-strong-reviewed", false, "confirm protected consumer now requests and verifies per-action assurance")
	flag.Parse()
	*client = strings.TrimSpace(*client)
	if !*reviewed || *client == "" {
		fail(fmt.Errorf("--client-id and --reviewed are required; select one cutover operation"))
	}
	if *retireStrong && *access != "" {
		fail(fmt.Errorf("perform access and assurance cutovers separately"))
	}
	if *access != "" && *access != "public" && *access != "private" {
		fail(fmt.Errorf("access must be public or private"))
	}
	if *access == "" && !*retireStrong {
		fail(fmt.Errorf("explicit cutover operation required"))
	}
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err = storesqlite.ApplyMigrations(ctx, db, migrations.FS); err != nil {
		fail(err)
	}
	if *retireStrong {
		if err = storesqlite.NewStore(db).RetireApplicationStrong(ctx, *client); err != nil {
			fail(err)
		}
	}
	if *access != "" {
		if err = storesqlite.NewStore(db).CutoverFederationAccess(ctx, *client, *access); err != nil {
			fail(err)
		}
	}
	fmt.Println("Reviewed cutover complete.")
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
