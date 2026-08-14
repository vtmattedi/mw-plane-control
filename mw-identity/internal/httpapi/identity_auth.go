package httpapi

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/portal"
)

const (
	sessionCookieName   = "mw_identity_session"
	csrfCookieName      = "mw_identity_csrf"
	loginCSRFCookieName = "mw_identity_login_csrf"
)

type IdentityAuthConfig struct {
	AppEnv                 string
	LoginCSRFTTL           time.Duration
	LoginRateLimitAttempts int
	LoginRateLimitWindow   time.Duration
	StrongAuthMaxAge       time.Duration
}

type identityAuthHandlers struct {
	service *auth.Service
	config  IdentityAuthConfig
	limiter *loginLimiter
	secure  bool
}

func newIdentityAuthHandlers(
	service *auth.Service,
	config IdentityAuthConfig,
) *identityAuthHandlers {
	return &identityAuthHandlers{
		service: service,
		config:  config,
		limiter: newLoginLimiter(
			config.LoginRateLimitAttempts,
			config.LoginRateLimitWindow,
		),
		secure: secureCookies(config.AppEnv),
	}
}

func (h *identityAuthHandlers) root(
	w http.ResponseWriter,
	r *http.Request,
) {
	if _, _, err := h.currentSession(r); err == nil {
		http.Redirect(
			w,
			r,
			"/account",
			http.StatusSeeOther,
		)
		return
	}

	http.Redirect(
		w,
		r,
		"/login",
		http.StatusSeeOther,
	)
}

func (h *identityAuthHandlers) loginPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	requestedAuth := strings.TrimSpace(r.URL.Query().Get("auth"))
	if _, _, err := h.currentSession(r); err == nil && requestedAuth == "" {
		http.Redirect(w, r, "/account", http.StatusSeeOther)
		return
	}

	token, err := auth.NewOpaqueToken("login_csrf_")
	if err != nil {
		writePlainError(
			w,
			http.StatusInternalServerError,
			"could not initialize login",
		)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     loginCSRFCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.config.LoginCSRFTTL.Seconds()),
		HttpOnly: false,
		Secure:   h.secure,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	if err := portal.RenderLogin(
		w,
		portal.LoginData{
			CSRFToken: token,
			AuthMode:  requestedAuth,
			ReturnTo: safeReturnTo(
				r.URL.Query().Get("return_to"),
			),
		},
	); err != nil {
		return
	}
}

func (h *identityAuthHandlers) login(
	w http.ResponseWriter,
	r *http.Request,
) {
	if !h.limiter.Allow(remoteIP(r)) {
		w.Header().Set(
			"Retry-After",
			strconv.Itoa(
				int(h.config.LoginRateLimitWindow.Seconds()),
			),
		)
		writePlainError(
			w,
			http.StatusTooManyRequests,
			"too many login attempts",
		)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		writePlainError(
			w,
			http.StatusBadRequest,
			"invalid login form",
		)
		return
	}

	returnTo := safeReturnTo(
		r.FormValue("return_to"),
	)

	if !h.validLoginCSRF(
		r,
		r.FormValue("csrf_token"),
	) {
		writePlainError(
			w,
			http.StatusForbidden,
			"invalid login request",
		)
		return
	}

	password := []byte(r.FormValue("password"))
	defer auth.ZeroBytes(password)

	result, err := h.service.Login(
		r.Context(),
		r.FormValue("email"),
		password,
		remoteIP(r),
	)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidLogin),
			errors.Is(err, auth.ErrUserDisabled),
			errors.Is(err, auth.ErrRecoveryPending):
			h.renderLoginFailure(
				w,
				"Invalid email or password.",
				returnTo,
			)
		default:
			writePlainError(
				w,
				http.StatusInternalServerError,
				"login failed",
			)
		}
		return
	}

	h.limiter.Reset(remoteIP(r))

	setSessionCookie(
		w,
		sessionCookieName,
		result.SessionToken,
		result.ExpiresAt,
		h.secure,
		true,
	)
	setSessionCookie(
		w,
		csrfCookieName,
		result.CSRFToken,
		result.ExpiresAt,
		h.secure,
		false,
	)
	clearCookie(
		w,
		loginCSRFCookieName,
		h.secure,
		false,
	)

	if returnTo == "" {
		returnTo = "/account"
	}

	http.Redirect(
		w,
		r,
		returnTo,
		http.StatusSeeOther,
	)
}

func (h *identityAuthHandlers) account(
	w http.ResponseWriter,
	r *http.Request,
) {
	sessionToken, result, err := h.currentSession(r)
	if err != nil {
		h.clearAuthCookies(w)
		http.Redirect(
			w,
			r,
			"/login",
			http.StatusSeeOther,
		)
		return
	}

	csrfToken := cookieValue(r, csrfCookieName)
	if csrfToken == "" ||
		!auth.TokenMatchesHash(
			csrfToken,
			result.Session.CSRFHash,
		) {
		csrfToken, err = h.service.RotateCSRF(
			r.Context(),
			sessionToken,
		)
		if err != nil {
			h.clearAuthCookies(w)
			http.Redirect(
				w,
				r,
				"/login",
				http.StatusSeeOther,
			)
			return
		}

		setSessionCookie(
			w,
			csrfCookieName,
			csrfToken,
			result.Session.ExpiresAt,
			h.secure,
			false,
		)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	_ = portal.RenderAccount(
		w,
		portal.AccountData{
			Email:        result.User.Email,
			DisplayName:  result.User.DisplayName,
			IsSuperadmin: result.User.IsSuperadmin,
			CSRFToken:    csrfToken,
		},
	)
}

func (h *identityAuthHandlers) logout(
	w http.ResponseWriter,
	r *http.Request,
) {
	sessionToken, result, err := h.currentSession(r)
	if err != nil {
		h.clearAuthCookies(w)
		http.Redirect(
			w,
			r,
			"/login",
			http.StatusSeeOther,
		)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	if err := r.ParseForm(); err != nil {
		writePlainError(
			w,
			http.StatusBadRequest,
			"invalid logout form",
		)
		return
	}

	csrfCookie := cookieValue(r, csrfCookieName)
	csrfForm := r.FormValue("csrf_token")

	if csrfCookie == "" ||
		!auth.TokensEqual(csrfCookie, csrfForm) ||
		!auth.TokenMatchesHash(
			csrfCookie,
			result.Session.CSRFHash,
		) {
		writePlainError(
			w,
			http.StatusForbidden,
			"invalid csrf token",
		)
		return
	}

	if err := h.service.Logout(
		r.Context(),
		sessionToken,
		remoteIP(r),
	); err != nil {
		writePlainError(
			w,
			http.StatusInternalServerError,
			"logout failed",
		)
		return
	}

	h.clearAuthCookies(w)
	http.Redirect(
		w,
		r,
		"/login",
		http.StatusSeeOther,
	)
}

func (h *identityAuthHandlers) currentSession(
	r *http.Request,
) (
	string,
	auth.SessionResult,
	error,
) {
	raw := cookieValue(r, sessionCookieName)
	if raw == "" {
		return "", auth.SessionResult{}, auth.ErrInvalidSession
	}

	result, err := h.service.Session(r.Context(), raw)
	if err != nil {
		return "", auth.SessionResult{}, err
	}

	return raw, result, nil
}

func (h *identityAuthHandlers) validLoginCSRF(
	r *http.Request,
	formToken string,
) bool {
	cookieToken := cookieValue(
		r,
		loginCSRFCookieName,
	)
	if cookieToken == "" || formToken == "" {
		return false
	}

	return auth.TokensEqual(cookieToken, formToken)
}

func (h *identityAuthHandlers) renderLoginFailure(
	w http.ResponseWriter,
	message string,
	returnTo string,
) {
	token, err := auth.NewOpaqueToken("login_csrf_")
	if err != nil {
		writePlainError(
			w,
			http.StatusUnauthorized,
			message,
		)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     loginCSRFCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.config.LoginCSRFTTL.Seconds()),
		HttpOnly: false,
		Secure:   h.secure,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)

	_ = portal.RenderLogin(
		w,
		portal.LoginData{
			CSRFToken: token,
			ReturnTo:  returnTo,
			Error:     message,
		},
	)
}

func (h *identityAuthHandlers) clearAuthCookies(
	w http.ResponseWriter,
) {
	clearCookie(
		w,
		sessionCookieName,
		h.secure,
		true,
	)
	clearCookie(
		w,
		csrfCookieName,
		h.secure,
		false,
	)
}

func setSessionCookie(
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

func secureCookies(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "", "development", "local", "test":
		return false
	default:
		return true
	}
}

func remoteIP(r *http.Request) string {
	if value := strings.TrimSpace(
		r.Header.Get("X-Real-IP"),
	); value != "" {
		return value
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func writePlainError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	w.Header().Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = fmt.Fprintln(w, message)
}
