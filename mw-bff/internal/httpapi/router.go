package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/delegation"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/identityclient"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/mwmail"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/mwoauth"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

func New(
	db *sql.DB,
	identity *identityclient.Client,
	sessions *session.Service,
	delegationClient *delegation.Client,
	delegationProbeConfig DelegationProbeConfig,
	oauthClient *mwoauth.Client,
	oauthAudience string,
	mailClient *mwmail.Client,
	mailAudience string,
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

	delegationProbe := newDelegationProbe(
		delegationClient,
		sessions,
		delegationProbeConfig,
	)

	oauthAdmin := newOAuthAdminProxy(
		delegationClient,
		sessions,
		oauthClient,
		oauthAudience,
		authConfig.StrongAuthMaxAge,
	)

	mailAdmin := newMailAdminProxy(
		delegationClient,
		sessions,
		mailClient,
		mailAudience,
		authConfig.StrongAuthMaxAge,
	)

	proxyClient := &http.Client{
		Timeout: authConfig.IdentityHTTPTimeout,
		CheckRedirect: func(
			req *http.Request,
			via []*http.Request,
		) error {
			return http.ErrUseLastResponse
		},
	}

	identityProxy := newIdentityProxy(
		sessions,
		authConfig.IdentityInternalBaseURL,
		proxyClient,
	)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		healthHandler("mw-bff", db),
	)
	mux.HandleFunc(
		"GET /ready",
		readinessHandler("mw-bff", db),
	)

	metrics := &operationalMetrics{}
	started := time.Now().UTC()
	mux.HandleFunc(
		"GET /metrics",
		metricsHandler("mw-bff", metrics, started),
	)

	if delegationProbe != nil {
		mux.HandleFunc(
			"GET /_test/delegation",
			func(w http.ResponseWriter, r *http.Request) {
				delegationProbe.serve(authHandlers, w, r)
			},
		)
	}

	if oauthAdmin != nil {
		mux.HandleFunc(
			"/oauth/{path...}",
			func(w http.ResponseWriter, r *http.Request) {
				oauthAdmin.serve(authHandlers, w, r)
			},
		)
	}

	if mailAdmin != nil {
		mux.HandleFunc(
			"/mail/{path...}",
			func(w http.ResponseWriter, r *http.Request) {
				mailAdmin.serve(authHandlers, w, r)
			},
		)
	}

	mux.HandleFunc(
		"GET /auth/login",
		authHandlers.login,
	)
	mux.HandleFunc(
		"GET /auth/callback",
		authHandlers.callback,
	)
	mux.HandleFunc("GET /auth/step-up", authHandlers.stepUp)
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

	mux.HandleFunc(
		"/identity/{path...}",
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			identityProxy.serve(
				authHandlers,
				w,
				r,
			)
		},
	)

	return operationalMiddleware(metrics, securityHeaders(mux))
}
