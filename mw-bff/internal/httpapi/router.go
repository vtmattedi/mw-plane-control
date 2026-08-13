package httpapi

import (
	"database/sql"
	"net/http"
)

func New(db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler("mw-bff", db))

	return securityHeaders(mux)
}
