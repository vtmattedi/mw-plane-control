package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/httpapi"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func main() {
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}),
	)
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}

	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		slog.Error("database open failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	migrationCtx, cancelMigrations := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	if err := storesqlite.ApplyMigrations(
		migrationCtx,
		db,
		migrations.FS,
	); err != nil {
		cancelMigrations()
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	cancelMigrations()

	store := storesqlite.NewStore(db)

	authService, err := auth.NewService(
		store,
		store,
		auth.ServiceConfig{
			SessionTTL: cfg.IdentitySessionTTL,
		},
	)
	if err != nil {
		slog.Error("identity auth initialization failed", "error", err)
		os.Exit(1)
	}

	keyring, err := signing.ParseKeyring(
		cfg.SigningKeys,
		cfg.SigningActiveKeyVersion,
	)
	if err != nil {
		slog.Error("signing encryption configuration failed", "error", err)
		os.Exit(1)
	}

	signingManager, err := signing.NewManager(store, keyring)
	if err != nil {
		slog.Error("signing manager initialization failed", "error", err)
		os.Exit(1)
	}

	signingCtx, cancelSigning := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	activeKID, err := signingManager.EnsureActiveKey(signingCtx)
	cancelSigning()
	if err != nil {
		slog.Error("active signing key initialization failed", "error", err)
		os.Exit(1)
	}

	oidcService, err := oidc.NewService(
		store,
		signingManager,
		oidc.ServiceConfig{
			Issuer:               cfg.PublicIssuer,
			AuthorizationCodeTTL: cfg.AuthorizationCodeTTL,
			AccessTokenTTL:       cfg.AccessTokenTTL,
			IDTokenTTL:           cfg.IDTokenTTL,
		},
	)
	if err != nil {
		slog.Error("OIDC service initialization failed", "error", err)
		os.Exit(1)
	}

	identityAdminService, err :=
		identityadmin.New(store)
	if err != nil {
		slog.Error(
			"Identity admin initialization failed",
			"error",
			err,
		)
		os.Exit(1)
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: httpapi.New(
			db,
			authService,
			httpapi.IdentityAuthConfig{
				AppEnv:                 cfg.AppEnv,
				LoginCSRFTTL:           cfg.LoginCSRFTTL,
				LoginRateLimitAttempts: cfg.LoginRateLimitAttempts,
				LoginRateLimitWindow:   cfg.LoginRateLimitWindow,
			},
			httpapi.OIDCMetadataConfig{
				PublicIssuer:   cfg.PublicIssuer,
				SigningManager: signingManager,
			},
			oidcService,
			identityAdminService,
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		<-shutdownCtx.Done()
		ctx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			slog.Error("server shutdown failed", "error", err)
		}
	}()

	slog.Info(
		"server starting",
		"service", "mw-identity",
		"app_env", cfg.AppEnv,
		"port", cfg.Port,
		"public_issuer", cfg.PublicIssuer,
		"signing_kid", activeKID,
	)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped", "service", "mw-identity")
}
