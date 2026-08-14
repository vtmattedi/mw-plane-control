package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MaintenanceStats struct {
	OIDCTransactions int64
	AdminSessions    int64
	DelegatedTokens  int64
}

func RunMaintenance(ctx context.Context, db *sql.DB, now time.Time) (MaintenanceStats, error) {
	nowUnix := now.UTC().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return MaintenanceStats{}, err
	}
	defer tx.Rollback()
	stats := MaintenanceStats{}
	ops := []struct {
		query string
		out   *int64
	}{
		{"DELETE FROM bff_oidc_transactions WHERE expires_at_unix < ?", &stats.OIDCTransactions},
		{"DELETE FROM bff_delegated_tokens WHERE expires_at_unix < ?", &stats.DelegatedTokens},
		{"DELETE FROM bff_admin_sessions WHERE expires_at_unix < ?", &stats.AdminSessions},
	}
	for _, op := range ops {
		result, err := tx.ExecContext(ctx, op.query, nowUnix)
		if err != nil {
			return MaintenanceStats{}, err
		}
		if *op.out, err = result.RowsAffected(); err != nil {
			return MaintenanceStats{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return MaintenanceStats{}, err
	}
	return stats, nil
}

func BackupDatabase(ctx context.Context, db *sql.DB, output string) error {
	output = strings.TrimSpace(output)
	if output == "" {
		return fmt.Errorf("backup output path is required")
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("backup output already exists: %s", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
		return fmt.Errorf("checkpoint WAL: %w", err)
	}
	escaped := strings.ReplaceAll(output, "'", "''")
	if _, err := db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'"); err != nil {
		return fmt.Errorf("VACUUM INTO: %w", err)
	}
	if err := os.Chmod(output, 0o600); err != nil {
		return err
	}
	return nil
}

func VerifyDatabase(ctx context.Context, path string, requiredTables []string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		return err
	}
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return err
	}
	if strings.TrimSpace(strings.ToLower(integrity)) != "ok" {
		return fmt.Errorf("integrity_check: %s", integrity)
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("foreign_key_check reported violations")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, table := range requiredTables {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("required table missing: %s", table)
		}
	}
	return nil
}
