package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitywebauthn"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/portal"
	webstatic "github.com/vtmattedi/mw-plane-control/mw-identity/web/static"
)

func New(
	db *sql.DB,
	authService *auth.Service,
	authConfig IdentityAuthConfig,
	oidcConfig OIDCMetadataConfig,
	oidcService *oidc.Service,
	identityAdminService *identityadmin.Service,
	identityWebAuthnService *identitywebauthn.Service,
	identityStrongService *identitystrong.Service,
	strongMaxAge time.Duration,
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
	if identityWebAuthnService == nil {
		panic(fmt.Errorf("Identity WebAuthn service is required"))
	}
	if identityStrongService == nil {
		panic(fmt.Errorf("Identity strong-auth service is required"))
	}

	authHandlers := newIdentityAuthHandlers(authService, authConfig)
	oauthHandlers := newOAuthHandlers(oidcService, authHandlers, identityWebAuthnService, identityStrongService)
	adminHandlers := newIdentityAdminHandlers(oidcService, identityAdminService, identityStrongService, authService, strongMaxAge)
	webAuthnHandlers := newIdentityWebAuthnHandlers(identityWebAuthnService, authHandlers, strongMaxAge)
	strongHandlers := newIdentityStrongHandlers(identityStrongService, identityWebAuthnService, authHandlers, strongMaxAge)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler("mw-identity", db),
	)
	mux.HandleFunc(
		"GET /ready",
		readinessHandler("mw-identity", db),
	)

	metrics := &operationalMetrics{}
	started := time.Now().UTC()
	mux.HandleFunc(
		"GET /metrics",
		metricsHandler("mw-identity", metrics, started),
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
	mux.HandleFunc("POST /oauth/introspect", oauthHandlers.introspect)

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
		"DELETE /admin/api/applications/{id}",
		adminHandlers.deleteApplication,
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
		"POST /admin/api/users",
		adminHandlers.createUser,
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
	mux.HandleFunc("POST /admin/api/users/{user_id}/revoke-sessions", adminHandlers.revokeUserSessions)
	mux.HandleFunc("DELETE /admin/api/users/{user_id}", adminHandlers.deleteUser)

	mux.HandleFunc("GET /webauthn-login.js", webAuthnHandlers.loginScript)
	mux.HandleFunc("GET /webauthn-account.js", webAuthnHandlers.accountScript)
	mux.HandleFunc("GET /api/webauthn/credentials", webAuthnHandlers.list)
	mux.HandleFunc("POST /api/webauthn/register/options", webAuthnHandlers.registerOptions)
	mux.HandleFunc("POST /api/webauthn/register/verify", webAuthnHandlers.registerVerify)
	mux.HandleFunc("POST /api/webauthn/login/options", webAuthnHandlers.loginOptions)
	mux.HandleFunc("POST /api/webauthn/login/verify", webAuthnHandlers.loginVerify)
	mux.HandleFunc("DELETE /api/webauthn/credentials/{credential_id}", webAuthnHandlers.remove)

	mux.HandleFunc("GET /api/strong/status", strongHandlers.status)
	mux.HandleFunc("POST /api/totp/setup", strongHandlers.setupTOTP)
	mux.HandleFunc("POST /api/totp/confirm", strongHandlers.confirmTOTP)
	mux.HandleFunc("POST /api/totp/disable", strongHandlers.disableTOTP)
	mux.HandleFunc("POST /api/totp/login", strongHandlers.loginTOTP)
	mux.HandleFunc("GET /recover", strongHandlers.recoveryPage)
	mux.HandleFunc("GET /recovery.js", strongHandlers.recoveryScript)
	mux.HandleFunc("POST /api/recovery/webauthn/options", strongHandlers.recoveryOptions)
	mux.HandleFunc("POST /api/recovery/webauthn/verify", strongHandlers.recoveryVerify)
	mux.HandleFunc("GET /strong-account.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write([]byte(portal.IdentityStrongAccountJS))
	})
	mux.HandleFunc("GET /totp-login.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write([]byte(portal.IdentityTOTPLoginJS))
	})

	mux.HandleFunc("GET /{$}", authHandlers.root)
	mux.HandleFunc("GET /login", authHandlers.loginPage)
	mux.HandleFunc("GET /login/en", authHandlers.loginPage)
	mux.HandleFunc("POST /login", authHandlers.login)
	mux.HandleFunc("GET /register", authHandlers.registerPage)
	mux.HandleFunc("GET /register/en", authHandlers.registerPage)
	mux.HandleFunc("POST /register", authHandlers.register)
	mux.HandleFunc("GET /continue", authHandlers.continuePage)
	mux.Handle("GET /static/", http.StripPrefix("/static/", webstatic.AssetHandler()))
	mux.HandleFunc("GET /account", authHandlers.account)
	mux.HandleFunc("GET /privacy", authHandlers.privacy)
	mux.HandleFunc("GET /security", authHandlers.security)
	mux.HandleFunc("POST /logout", authHandlers.logout)
	mux.HandleFunc("POST /oauth/consent", oauthHandlers.consent)
	mux.HandleFunc("POST /account/delete", authHandlers.deleteAccount)
	mux.HandleFunc("POST /account/applications/{id}/remove", authHandlers.removeApplication)

	return operationalMiddleware(metrics, securityHeaders(mux))
}
