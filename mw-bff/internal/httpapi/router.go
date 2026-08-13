package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/identityclient"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

func New(
	db *sql.DB,
	identity *identityclient.Client,
	sessions *session.Service,
	authConfig AuthConfig,
) http.Handler {
	if identity == nil {
		panic(fmt.Errorf(
			"Identity OIDC client is required",
		))
	}
	if sessions == nil {
		panic(fmt.Errorf(
			"admin session service is required",
		))
	}

	authHandlers := newAuthHandlers(
		identity,
		sessions,
		authConfig,
	)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler("mw-bff", db),
	)

	mux.HandleFunc(
		"GET /auth/login",
		authHandlers.login,
	)
	mux.HandleFunc(
		"GET /auth/callback",
		authHandlers.callback,
	)
	mux.HandleFunc(
		"GET /auth/me",
		authHandlers.me,
	)
	mux.HandleFunc(
		"POST /auth/csrf",
		authHandlers.csrf,
	)
	mux.HandleFunc(
		"POST /auth/logout",
		authHandlers.logout,
	)

	return securityHeaders(mux)
}
