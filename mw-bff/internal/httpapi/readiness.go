package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type readinessSQLite struct {
	ForeignKeys   bool   `json:"foreign_keys"`
	JournalMode   string `json:"journal_mode"`
	BusyTimeoutMS int    `json:"busy_timeout_ms"`
}

type readinessResponse struct {
	Status   string          `json:"status"`
	Service  string          `json:"service"`
	Database string          `json:"database"`
	SQLite   readinessSQLite `json:"sqlite"`
}

func readinessHandler(service string, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()

		response := readinessResponse{Status: "ready", Service: service, Database: "ready"}
		status := http.StatusOK

		var foreignKeys int
		var journalMode string
		var busyTimeout int
		if err := db.PingContext(ctx); err != nil ||
			db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys) != nil ||
			db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode) != nil ||
			db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout) != nil {
			status = http.StatusServiceUnavailable
			response.Status = "not_ready"
			response.Database = "unavailable"
		} else {
			response.SQLite = readinessSQLite{
				ForeignKeys:   foreignKeys == 1,
				JournalMode:   strings.ToLower(strings.TrimSpace(journalMode)),
				BusyTimeoutMS: busyTimeout,
			}
			if !response.SQLite.ForeignKeys || response.SQLite.JournalMode != "wal" || busyTimeout < 1000 {
				status = http.StatusServiceUnavailable
				response.Status = "not_ready"
				response.Database = "misconfigured"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(response)
	}
}
