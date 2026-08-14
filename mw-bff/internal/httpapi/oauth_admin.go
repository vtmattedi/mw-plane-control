package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/delegation"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/mwoauth"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

const maxOAuthAdminRequestBytes int64 = 1024 * 1024

type oauthAdminProxy struct {
	delegation   *delegation.Client
	sessions     *session.Service
	oauth        *mwoauth.Client
	audience     string
	strongMaxAge time.Duration
}

func newOAuthAdminProxy(
	delegationClient *delegation.Client,
	sessions *session.Service,
	oauthClient *mwoauth.Client,
	audience string,
	strongMaxAge time.Duration,
) *oauthAdminProxy {
	audience = strings.TrimSpace(audience)
	if delegationClient == nil || sessions == nil || oauthClient == nil || audience == "" {
		return nil
	}
	return &oauthAdminProxy{
		delegation:   delegationClient,
		sessions:     sessions,
		oauth:        oauthClient,
		audience:     audience,
		strongMaxAge: strongMaxAge,
	}
}

func (p *oauthAdminProxy) serve(
	auth *authHandlers,
	w http.ResponseWriter,
	r *http.Request,
) {
	noStore(w)

	_, record, err := auth.current(r)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	if requiresCSRF(r.Method) && !validAdminCSRF(r, record) {
		writeJSONError(w, http.StatusForbidden, "invalid_csrf")
		return
	}

	route, err := mwoauth.Resolve(r.Method, "/"+r.PathValue("path"))
	if errors.Is(err, mwoauth.ErrRouteNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_oauth_route")
		return
	}
	if sensitiveOAuthRoute(r.Method, route.DownstreamPath) && !requireBFFWebAuthn(w, record, p.strongMaxAge) {
		return
	}

	body, err := readOAuthAdminBody(w, r)
	if err != nil {
		if errors.Is(err, errOAuthRequestTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request_too_large")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid_request_body")
		return
	}

	subjectToken, err := p.sessions.IdentityAccessToken(record)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "identity_session_unavailable")
		return
	}
	defer zeroStringBestEffort(&subjectToken)

	delegated, err := p.delegation.Token(
		r.Context(),
		record.ID,
		subjectToken,
		p.audience,
		[]string{route.Scope},
	)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "oauth_delegation_failed")
		return
	}
	defer zeroStringBestEffort(&delegated.AccessToken)

	response, err := p.oauth.Do(
		r.Context(),
		r.Method,
		route.DownstreamPath,
		r.URL.RawQuery,
		delegated.AccessToken,
		r.Header.Get("Content-Type"),
		r.Header.Get("Accept"),
		r.Header.Get("X-Request-ID"),
		body,
	)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "oauth_upstream_unavailable")
		return
	}

	if delegated.Cached {
		w.Header().Set("X-MW-Delegation-Cache", "hit")
	} else {
		w.Header().Set("X-MW-Delegation-Cache", "miss")
	}
	if response.RequestID != "" {
		w.Header().Set("X-Request-ID", response.RequestID)
	}

	// A downstream 401 means the control-plane federation wiring was rejected,
	// not that the browser lacks a BFF session. Do not leak the downstream body.
	if response.StatusCode == http.StatusUnauthorized {
		writeJSONError(w, http.StatusBadGateway, "oauth_federation_rejected")
		return
	}
	if response.StatusCode >= 500 {
		writeJSONError(w, http.StatusBadGateway, "oauth_upstream_failed")
		return
	}

	if response.ContentType != "" {
		w.Header().Set("Content-Type", response.ContentType)
	}
	w.WriteHeader(response.StatusCode)
	if len(response.Body) > 0 {
		_, _ = w.Write(response.Body)
	}
}

var errOAuthRequestTooLarge = errors.New("OAuth admin request too large")

func readOAuthAdminBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	limited := http.MaxBytesReader(w, r.Body, maxOAuthAdminRequestBytes)
	defer limited.Close()
	body, err := io.ReadAll(limited)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, errOAuthRequestTooLarge
		}
		return nil, err
	}
	return body, nil
}

func validAdminCSRF(r *http.Request, record session.Record) bool {
	csrfCookie := cookieValue(r, adminCSRFCookieName)
	csrfHeader := strings.TrimSpace(r.Header.Get(adminCSRFHeaderName))
	return csrfCookie != "" &&
		csrfHeader != "" &&
		session.TokensEqual(csrfCookie, csrfHeader) &&
		session.TokenMatchesHash(csrfCookie, record.CSRFHash)
}
