package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func main() {
	email := flag.String(
		"email",
		"",
		"superadmin email",
	)
	displayName := flag.String(
		"display-name",
		"",
		"display name",
	)
	passwordStdin := flag.Bool(
		"password-stdin",
		false,
		"read one password line from stdin",
	)
	flag.Parse()

	if strings.TrimSpace(*email) == "" {
		fatal("email is required")
	}

	password, err := readPassword(*passwordStdin)
	if err != nil {
		fatal(err.Error())
	}
	defer auth.ZeroBytes(password)

	cfg, err := config.Load()
	if err != nil {
		fatal(fmt.Sprintf("configuration failed: %v", err))
	}

	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		fatal(fmt.Sprintf("database open failed: %v", err))
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	if err := storesqlite.ApplyMigrations(
		ctx,
		db,
		migrations.FS,
	); err != nil {
		fatal(fmt.Sprintf("database migration failed: %v", err))
	}

	store := storesqlite.NewStore(db)
	service, err := auth.NewService(
		store,
		store,
		auth.ServiceConfig{
			SessionTTL: cfg.IdentitySessionTTL,
		},
	)
	if err != nil {
		fatal(fmt.Sprintf("auth initialization failed: %v", err))
	}

	user, err := service.CreateSuperadmin(
		ctx,
		*email,
		*displayName,
		password,
	)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrEmailExists):
			fatal("an identity user with that email already exists")
		case errors.Is(err, auth.ErrPasswordTooShort):
			fatal(err.Error())
		default:
			fatal(fmt.Sprintf("create superadmin failed: %v", err))
		}
	}

	fmt.Printf(
		"created superadmin %s (%s)\n",
		user.Email,
		user.ID,
	)
}

func readPassword(fromStdin bool) ([]byte, error) {
	if fromStdin {
		reader := bufio.NewReader(io.LimitReader(os.Stdin, 16*1024))
		value, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read password stdin: %w", err)
		}

		value = strings.TrimRight(value, "\r\n")
		return []byte(value), nil
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf(
			"stdin is not a terminal; use --password-stdin",
		)
	}

	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}

	fmt.Fprint(os.Stderr, "Confirm password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		auth.ZeroBytes(first)
		return nil, fmt.Errorf("read password confirmation: %w", err)
	}
	defer auth.ZeroBytes(second)

	if !auth.TokensEqual(string(first), string(second)) {
		auth.ZeroBytes(first)
		return nil, fmt.Errorf("passwords do not match")
	}

	return first, nil
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "ERROR:", message)
	os.Exit(1)
}
