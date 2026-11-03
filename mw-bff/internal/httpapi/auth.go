package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/identityclient"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

const (
	adminSessionCookieName = "mw_admin_session"
	adminCSRFCookieName    = "mw_admin_csrf"
	adminCSRFHeaderName    = "X-MW-CSRF"
)

type AuthConfig struct {
	AppEnv                  string
	PublicBaseURL           string
	IdentityInternalBaseURL string
	IdentityHTTPTimeout     time.Duration
	StrongAuthMaxAge        time.Duration
}

type authHandlers struct {
	identity *identityclient.Client
	sessions *session.Service
	config   AuthConfig
	secure   bool
}

func newAuthHandlers(
	identity *identityclient.Client,
	sessions *session.Service,
	config AuthConfig,
) *authHandlers {
	return &authHandlers{
		identity: identity,
		sessions: sessions,
		config:   config,
		secure:   secureCookies(config.AppEnv),
	}
}

func (h *authHandlers) login(
	w http.ResponseWriter,
	r *http.Request,
) {
	returnTo := safeReturnTo(
		r.URL.Query().Get("return_to"),
	)

	target, err := h.identity.Start(
		r.Context(),
		returnTo,
	)
	if err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"login_start_failed",
		)
		return
	}

	noStore(w)
	http.Redirect(
		w,
		r,
		target,
		http.StatusFound,
	)
}

func (h *authHandlers) stepUp(w http.ResponseWriter, r *http.Request) {
	_, record, err := h.current(r)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	_ = record
	returnTo := safeReturnTo(r.URL.Query().Get("return_to"))
	target, err := h.identity.StartStepUp(r.Context(), returnTo, record.IdentitySubject)
	if err != nil {
		writeJSONError(w, 500, "step_up_start_failed")
		return
	}
	noStore(w)
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *authHandlers) callback(
	w http.ResponseWriter,
	r *http.Request,
) {
	noStore(w)

	if r.URL.Query().Get("error") != "" {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"identity_authorization_failed",
		)
		return
	}

	identity, returnTo, err :=
		h.identity.Complete(
			r.Context(),
			r.URL.Query().Get("code"),
			r.URL.Query().Get("state"),
		)
	if err != nil {
		slog.Error(
			"Identity OIDC callback failed",
			"error", err,
		)
		status := http.StatusBadGateway
		code := "identity_callback_failed"

		if errors.Is(
			err,
			identityclient.ErrTransactionNotFound,
		) {
			status = http.StatusBadRequest
			code = "invalid_or_expired_state"
		}

		writeJSONError(w, status, code)
		return
	}

	created, err := h.sessions.Create(
		r.Context(),
		session.Identity{
			Issuer:               identity.Issuer,
			Subject:              identity.Subject,
			Email:                identity.Email,
			DisplayName:          identity.DisplayName,
			AuthTime:             identity.AuthTime,
			AuthMethod:           identity.AuthMethod,
			AccessToken:          identity.AccessToken,
			AccessTokenExpiresAt: identity.AccessTokenExpiresAt,
		},
	)
	if err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"session_creation_failed",
		)
		return
	}

	setCookie(
		w,
		adminSessionCookieName,
		created.RawSession,
		created.Record.ExpiresAt,
		h.secure,
		true,
	)
	setCookie(
		w,
		adminCSRFCookieName,
		created.RawCSRF,
		created.Record.ExpiresAt,
		h.secure,
		false,
	)

	if returnTo == "" {
		returnTo = "/"
	}

	http.Redirect(
		w,
		r,
		h.config.PublicBaseURL+returnTo,
		http.StatusSeeOther,
	)
}

func (h *authHandlers) me(
	w http.ResponseWriter,
	r *http.Request,
) {
	noStore(w)

	_, record, err := h.current(r)
	if err != nil {
		writeJSONError(
			w,
			http.StatusUnauthorized,
			"unauthenticated",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"identity": map[string]any{
				"issuer":      record.IdentityIssuer,
				"sub":         record.IdentitySubject,
				"email":       record.Email,
				"name":        record.DisplayName,
				"auth_time":   record.AuthTime.Unix(),
				"auth_method": record.AuthMethod,
				"assurance":   assuranceFor(record),
			},
			"session": map[string]any{
				"expires_at":      record.ExpiresAt.Unix(),
				"recent_webauthn": recentBFFWebAuthn(record, h.config.StrongAuthMaxAge, time.Now()),
			},
		},
	)
}

func (h *authHandlers) csrf(
	w http.ResponseWriter,
	r *http.Request,
) {
	noStore(w)

	rawSession, record, err := h.current(r)
	if err != nil {
		writeJSONError(
			w,
			http.StatusUnauthorized,
			"unauthenticated",
		)
		return
	}

	token, err := h.sessions.RotateCSRF(
		r.Context(),
		rawSession,
	)
	if err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"csrf_rotation_failed",
		)
		return
	}

	setCookie(
		w,
		adminCSRFCookieName,
		token,
		record.ExpiresAt,
		h.secure,
		false,
	)

	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"csrf_token": token,
		},
	)
}

func (h *authHandlers) logout(
	w http.ResponseWriter,
	r *http.Request,
) {
	noStore(w)

	rawSession, record, err := h.current(r)
	if err != nil {
		h.clearCookies(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}

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

	if err := h.sessions.Logout(
		r.Context(),
		rawSession,
	); err != nil {
		writeJSONError(
			w,
			http.StatusInternalServerError,
			"logout_failed",
		)
		return
	}

	h.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandlers) current(
	r *http.Request,
) (
	string,
	session.Record,
	error,
) {
	rawSession := cookieValue(
		r,
		adminSessionCookieName,
	)
	if rawSession == "" {
		return "", session.Record{}, session.ErrInvalid
	}

	record, err := h.sessions.Get(
		r.Context(),
		rawSession,
	)
	if err != nil {
		return "", session.Record{}, err
	}

	return rawSession, record, nil
}

func (h *authHandlers) clearCookies(
	w http.ResponseWriter,
) {
	clearCookie(
		w,
		adminSessionCookieName,
		h.secure,
		true,
	)
	clearCookie(
		w,
		adminCSRFCookieName,
		h.secure,
		false,
	)
}

func safeReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "//") ||
		strings.Contains(value, `\`) {
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil ||
		parsed.IsAbs() ||
		parsed.Host != "" ||
		!strings.HasPrefix(parsed.Path, "/") {
		return ""
	}

	return value
}

func secureCookies(appEnv string) bool {
	switch strings.ToLower(
		strings.TrimSpace(appEnv),
	) {
	case "", "development", "local", "test":
		return false
	default:
		return true
	}
}

func setCookie(
	w http.ResponseWriter,
	name string,
	value string,
	expires time.Time,
	secure bool,
	httpOnly bool,
) {
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}

	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   maxAge,
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearCookie(
	w http.ResponseWriter,
	name string,
	secure bool,
	httpOnly bool,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func cookieValue(
	r *http.Request,
	name string,
) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func noStore(w http.ResponseWriter) {
	w.Header().Set(
		"Cache-Control",
		"no-store, max-age=0",
	)
	w.Header().Set(
		"Pragma",
		"no-cache",
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(
	w http.ResponseWriter,
	status int,
	code string,
) {
	writeJSON(
		w,
		status,
		map[string]any{
			"error": map[string]string{
				"code": code,
			},
		},
	)
}
