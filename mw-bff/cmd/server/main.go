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

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/config"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/httpapi"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-bff/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-bff/migrations"
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

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpapi.New(db),
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
		"service", "mw-bff",
		"app_env", cfg.AppEnv,
		"port", cfg.Port,
	)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		slog.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped", "service", "mw-bff")
}
