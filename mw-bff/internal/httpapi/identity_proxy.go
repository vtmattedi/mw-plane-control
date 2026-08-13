package httpapi

import (
	"io"
	"net/http"
	"strings"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

type identityProxy struct {
	sessions    *session.Service
	internalURL string
	httpClient  *http.Client
}

func newIdentityProxy(
	sessions *session.Service,
	internalURL string,
	httpTimeoutClient *http.Client,
) *identityProxy {
	return &identityProxy{
		sessions: sessions,
		internalURL: strings.TrimRight(
			internalURL,
			"/",
		),
		httpClient: httpTimeoutClient,
	}
}

func (p *identityProxy) serve(
	auth *authHandlers,
	w http.ResponseWriter,
	r *http.Request,
) {
	noStore(w)

	rawSession, record, err := auth.current(r)
	if err != nil {
		writeJSONError(
			w,
			http.StatusUnauthorized,
			"unauthenticated",
		)
		return
	}

	if requiresCSRF(r.Method) {
		csrfCookie := cookieValue(
			r,
			adminCSRFCookieName,
		)
		csrfHeader := strings.TrimSpace(
			r.Header.Get(adminCSRFHeaderName),
		)

		if csrfCookie == "" ||
			csrfHeader == "" ||
			!session.TokensEqual(
				csrfCookie,
				csrfHeader,
			) ||
			!session.TokenMatchesHash(
				csrfCookie,
				record.CSRFHash,
			) {
			writeJSONError(
				w,
				http.StatusForbidden,
				"invalid_csrf",
			)
			return
		}
	}

	accessToken, err :=
		p.sessions.IdentityAccessToken(
			record,
		)
	if err != nil {
		writeJSONError(
			w,
			http.StatusUnauthorized,
			"identity_admin_session_unavailable",
		)
		return
	}
	defer zeroStringBestEffort(
		&accessToken,
	)

	rest := r.PathValue("path")
	if rest == "" ||
		strings.Contains(rest, `\\`) {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid_identity_path",
		)
		return
	}

	for _, segment := range strings.Split(rest, "/") {
		if segment == "." || segment == ".." {
			writeJSONError(
				w,
				http.StatusBadRequest,
				"invalid_identity_path",
			)
			return
		}
	}

	target := p.internalURL +
		"/admin/api/" +
		rest

	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	body := http.MaxBytesReader(
		w,
		r.Body,
		1024*1024,
	)

	request, err := http.NewRequestWithContext(
		r.Context(),
		r.Method,
		target,
		body,
	)
	if err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"identity_proxy_failed",
		)
		return
	}

	request.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	if contentType := r.Header.Get(
		"Content-Type",
	); contentType != "" {
		request.Header.Set(
			"Content-Type",
			contentType,
		)
	}

	if requestID := r.Header.Get(
		"X-Request-Id",
	); requestID != "" {
		request.Header.Set(
			"X-Request-Id",
			requestID,
		)
	}

	if realIP := strings.TrimSpace(
		r.Header.Get("X-Real-IP"),
	); realIP != "" {
		request.Header.Set(
			"X-Real-IP",
			realIP,
		)
	}

	response, err := p.httpClient.Do(request)
	if err != nil {
		writeJSONError(
			w,
			http.StatusBadGateway,
			"identity_unavailable",
		)
		return
	}
	defer response.Body.Close()

	for _, name := range []string{
		"Content-Type",
		"Cache-Control",
		"Pragma",
		"WWW-Authenticate",
	} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}

	w.WriteHeader(response.StatusCode)

	_, _ = io.Copy(
		w,
		io.LimitReader(
			response.Body,
			2*1024*1024,
		),
	)

	_ = rawSession
}

func requiresCSRF(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodOptions:
		return false
	default:
		return true
	}
}

func zeroStringBestEffort(value *string) {
	// Go strings are immutable; this only drops our reference promptly.
	// The decrypted bearer token is never logged or persisted in plaintext.
	*value = ""
}
