package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
)

func New(
	db *sql.DB,
	authService *auth.Service,
	authConfig IdentityAuthConfig,
	oidcConfig OIDCMetadataConfig,
	oidcService *oidc.Service,
) http.Handler {
	if oidcConfig.SigningManager == nil {
		panic(fmt.Errorf("OIDC signing manager is required"))
	}
	if oidcService == nil {
		panic(fmt.Errorf("OIDC service is required"))
	}

	authHandlers := newIdentityAuthHandlers(authService, authConfig)
	oauthHandlers := newOAuthHandlers(oidcService, authHandlers)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler("mw-identity", db),
	)
	mux.HandleFunc(
		"GET /.well-known/openid-configuration",
		discoveryHandler(oidcConfig.PublicIssuer),
	)
	mux.HandleFunc(
		"GET /.well-known/jwks.json",
		jwksHandler(oidcConfig.SigningManager),
	)

	mux.HandleFunc("GET /oauth/authorize", oauthHandlers.authorize)
	mux.HandleFunc("POST /oauth/token", oauthHandlers.token)
	mux.HandleFunc("GET /oauth/userinfo", oauthHandlers.userInfo)

	mux.HandleFunc("GET /{$}", authHandlers.root)
	mux.HandleFunc("GET /login", authHandlers.loginPage)
	mux.HandleFunc("POST /login", authHandlers.login)
	mux.HandleFunc("GET /account", authHandlers.account)
	mux.HandleFunc("POST /logout", authHandlers.logout)

	return securityHeaders(mux)
}
