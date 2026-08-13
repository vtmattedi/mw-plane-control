package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
)

func New(
	db *sql.DB,
	authService *auth.Service,
	authConfig IdentityAuthConfig,
	oidcConfig OIDCMetadataConfig,
	oidcService *oidc.Service,
	identityAdminService *identityadmin.Service,
) http.Handler {
	if oidcConfig.SigningManager == nil {
		panic(fmt.Errorf("OIDC signing manager is required"))
	}
	if oidcService == nil {
		panic(fmt.Errorf("OIDC service is required"))
	}
	if identityAdminService == nil {
		panic(fmt.Errorf("Identity admin service is required"))
	}

	authHandlers := newIdentityAuthHandlers(authService, authConfig)
	oauthHandlers := newOAuthHandlers(oidcService, authHandlers)
	adminHandlers := newIdentityAdminHandlers(
		oidcService,
		identityAdminService,
	)

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

	mux.HandleFunc(
		"GET /admin/api/whoami",
		adminHandlers.whoami,
	)
	mux.HandleFunc(
		"GET /admin/api/applications",
		adminHandlers.listApplications,
	)
	mux.HandleFunc(
		"POST /admin/api/applications",
		adminHandlers.createApplication,
	)
	mux.HandleFunc(
		"GET /admin/api/applications/{id}",
		adminHandlers.getApplication,
	)
	mux.HandleFunc(
		"PATCH /admin/api/applications/{id}",
		adminHandlers.updateApplication,
	)
	mux.HandleFunc(
		"POST /admin/api/applications/{id}/disable",
		adminHandlers.applicationState("disable"),
	)
	mux.HandleFunc(
		"POST /admin/api/applications/{id}/enable",
		adminHandlers.applicationState("enable"),
	)
	mux.HandleFunc(
		"POST /admin/api/applications/{id}/archive",
		adminHandlers.applicationState("archive"),
	)
	mux.HandleFunc(
		"POST /admin/api/applications/{id}/rotate-secret",
		adminHandlers.rotateSecret,
	)
	mux.HandleFunc(
		"GET /admin/api/users",
		adminHandlers.users,
	)
	mux.HandleFunc(
		"GET /admin/api/applications/{id}/assignments",
		adminHandlers.assignments,
	)
	mux.HandleFunc(
		"PUT /admin/api/applications/{id}/assignments/{user_id}",
		adminHandlers.upsertAssignment,
	)
	mux.HandleFunc(
		"DELETE /admin/api/applications/{id}/assignments/{user_id}",
		adminHandlers.deleteAssignment,
	)
	mux.HandleFunc(
		"GET /admin/api/audit",
		adminHandlers.audit,
	)

	mux.HandleFunc("GET /{$}", authHandlers.root)
	mux.HandleFunc("GET /login", authHandlers.loginPage)
	mux.HandleFunc("POST /login", authHandlers.login)
	mux.HandleFunc("GET /account", authHandlers.account)
	mux.HandleFunc("POST /logout", authHandlers.logout)

	return securityHeaders(mux)
}
