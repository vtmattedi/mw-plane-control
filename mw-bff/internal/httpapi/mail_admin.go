package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/delegation"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/mwmail"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

const maxMailAdminRequestBytes int64 = 1024 * 1024

type mailAdminProxy struct {
	delegation   *delegation.Client
	sessions     *session.Service
	mail         *mwmail.Client
	audience     string
	strongMaxAge time.Duration
}

func newMailAdminProxy(
	delegationClient *delegation.Client,
	sessions *session.Service,
	mailClient *mwmail.Client,
	audience string,
	strongMaxAge time.Duration,
) *mailAdminProxy {
	audience = strings.TrimSpace(audience)
	if delegationClient == nil || sessions == nil ||
		mailClient == nil || audience == "" {
		return nil
	}
	return &mailAdminProxy{
		delegation:   delegationClient,
		sessions:     sessions,
		mail:         mailClient,
		audience:     audience,
		strongMaxAge: strongMaxAge,
	}
}

func (p *mailAdminProxy) serve(
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

	route, err := mwmail.Resolve(r.Method, "/"+r.PathValue("path"))
	if errors.Is(err, mwmail.ErrRouteNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_mail_route")
		return
	}
	if sensitiveMailRoute(r.Method, route.DownstreamPath) && !requireBFFWebAuthn(w, record, p.strongMaxAge) {
		return
	}

	body, err := readMailAdminBody(w, r)
	if err != nil {
		if errors.Is(err, errMailRequestTooLarge) {
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
		writeJSONError(w, http.StatusBadGateway, "mail_delegation_failed")
		return
	}
	defer zeroStringBestEffort(&delegated.AccessToken)

	response, err := p.mail.Do(
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
		writeJSONError(w, http.StatusBadGateway, "mail_upstream_unavailable")
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

	// A downstream 401 means Mail rejected the control-plane federation
	// credentials/token. The browser still has a valid BFF session.
	if response.StatusCode == http.StatusUnauthorized {
		writeJSONError(w, http.StatusBadGateway, "mail_federation_rejected")
		return
	}
	if response.StatusCode >= 500 {
		writeJSONError(w, http.StatusBadGateway, "mail_upstream_failed")
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

var errMailRequestTooLarge = errors.New("Mail admin request too large")

func readMailAdminBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	limited := http.MaxBytesReader(w, r.Body, maxMailAdminRequestBytes)
	defer limited.Close()
	body, err := io.ReadAll(limited)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, errMailRequestTooLarge
		}
		return nil, err
	}
	return body, nil
}
