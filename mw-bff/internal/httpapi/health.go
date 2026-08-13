package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/buildinfo"
)

type pinger interface {
	PingContext(context.Context) error
}

type healthResponse struct {
	Status   string         `json:"status"`
	Service  string         `json:"service"`
	Database string         `json:"database"`
	Build    buildinfo.Info `json:"build"`
}

func healthHandler(
	service string,
	db pinger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
		defer cancel()

		response := healthResponse{
			Status:   "ok",
			Service:  service,
			Database: "ok",
			Build:    buildinfo.Current(),
		}

		status := http.StatusOK
		if err := db.PingContext(ctx); err != nil {
			response.Status = "unavailable"
			response.Database = "unavailable"
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(response)
	}
}
