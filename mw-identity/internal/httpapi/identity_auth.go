package httpapi

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	webstatic "github.com/vtmattedi/mw-plane-control/mw-identity/web/static"
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
	// PublicSignupEnabled allows /register without an application.
	PublicSignupEnabled bool
}

// Sign-up has its own budget so it can neither exhaust nor reset the login one.
const (
	registrationRateAttempts = 10
	registrationRateWindow   = time.Hour
)

type identityAuthHandlers struct {
	service             *auth.Service
	config              IdentityAuthConfig
	limiter             *loginLimiter
	registrationLimiter *loginLimiter
	secure              bool
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
		registrationLimiter: newLoginLimiter(registrationRateAttempts, registrationRateWindow),
		secure:              secureCookies(config.AppEnv),
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
	// Remember an explicit language choice first: a signed-in visitor is
	// redirected to the account page below, which should honor it.
	rememberLanguage(w, r, h.secure)
	requestedAuth := requestedLoginAuth(r.URL.Query().Get("auth"))
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

	returnTo := safeReturnTo(r.URL.Query().Get("return_to"))
	registrationURL := h.registrationURL(r, returnTo)
	if loginPageEnglish(r) {
		registrationURL = strings.Replace(registrationURL, "/register?", "/register/en?", 1)
	}
	// A plain visit to the login page offers stand-alone sign-up; a login
	// that returns somewhere else (an app, step-up) only offers the app's.
	if registrationURL == "" && returnTo == "" && requestedAuth == "" && h.config.PublicSignupEnabled {
		registrationURL = "/register"
		if loginPageEnglish(r) {
			registrationURL = "/register/en"
		}
	}
	restricted := false
	if target, e := url.Parse(returnTo); e == nil && target.Path == "/oauth/authorize" {
		if app, e := h.service.PublicSignupApplication(r.Context(), target.Query().Get("client_id")); e == nil {
			restricted = app.AccessPolicy == "default_deny"
		}
	}
	if err := webstatic.RenderLogin(
		w,
		loginPageEnglish(r),
		webstatic.LoginData{
			RestrictedApplication: restricted,
			CSRFToken:             token,
			ReturnTo:              returnTo,
			RegistrationURL:       registrationURL,
			RequiredAuth:          requestedAuth,
		},
	); err != nil {
		return
	}
}

func requestedLoginAuth(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "reauth":
		return "reauth"
	case "strong":
		return "strong"
	case "webauthn":
		return "webauthn"
	default:
		return ""
	}
}

func (h *identityAuthHandlers) registrationURL(r *http.Request, returnTo string) string {
	parsed, err := url.Parse(returnTo)
	if err != nil || parsed.Path != "/oauth/authorize" {
		return ""
	}
	clientID := parsed.Query().Get("client_id")
	if app, err := h.service.PublicSignupApplication(r.Context(), clientID); err != nil || app.AccessPolicy != "default_allow" || !h.config.PublicSignupEnabled {
		return ""
	}
	values := url.Values{"client_id": {clientID}, "return_to": {returnTo}}
	return "/register?" + values.Encode()
}

// registrationTarget validates where a sign-up came from. With a client_id it
// must be an authorization request for an application that allows public
// sign-up; without one it is a stand-alone sign-up, allowed only when enabled.
func (h *identityAuthHandlers) registrationTarget(r *http.Request, clientID, returnTo string) (webstatic.RegistrationData, bool) {
	if !h.config.PublicSignupEnabled {
		return webstatic.RegistrationData{}, false
	}
	if clientID == "" {
		return webstatic.RegistrationData{}, true
	}
	parsedReturn, _ := url.Parse(returnTo)
	app, err := h.service.PublicSignupApplication(r.Context(), clientID)
	if err != nil || parsedReturn == nil || parsedReturn.Path != "/oauth/authorize" || parsedReturn.Query().Get("client_id") != clientID {
		return webstatic.RegistrationData{}, false
	}
	return webstatic.RegistrationData{ReturnTo: returnTo, ClientID: clientID, AppName: app.Name}, true
}

// withRegistrationLinks fills the language-dependent links of the sign-up
// page. They are built from the form's own fields so a POST that re-renders
// the page (after an error) keeps the app's client_id and return_to.
func withRegistrationLinks(data webstatic.RegistrationData, lang string) webstatic.RegistrationData {
	data.Lang = lang
	other := "en"
	login := "/login"
	if lang == webstatic.LangEN {
		other = "pt"
		login = "/login/en"
	}
	query := url.Values{"lang": {other}}
	if data.ClientID != "" {
		query.Set("client_id", data.ClientID)
	}
	if data.ReturnTo != "" {
		query.Set("return_to", data.ReturnTo)
		login += "?return_to=" + url.QueryEscape(data.ReturnTo)
	}
	data.LangSwitchURL = "/register?" + query.Encode()
	data.LoginURL = login
	return data
}

func (h *identityAuthHandlers) registerPage(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	data, ok := h.registrationTarget(r, clientID, safeReturnTo(r.URL.Query().Get("return_to")))
	if !ok {
		http.NotFound(w, r)
		return
	}
	token, err := auth.NewOpaqueToken("login_csrf_")
	if err != nil {
		writePlainError(w, http.StatusInternalServerError, "could not initialize registration")
		return
	}
	h.setLoginCSRFCookie(w, token)
	rememberLanguage(w, r, h.secure)
	data = withRegistrationLinks(data, pageLanguage(r))
	data.CSRFToken = token
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = webstatic.RenderRegistration(w, data)
}

func (h *identityAuthHandlers) register(w http.ResponseWriter, r *http.Request) {
	if !h.registrationLimiter.Allow(remoteIP(r)) {
		w.Header().Set("Retry-After", strconv.Itoa(int(registrationRateWindow.Seconds())))
		writePlainError(w, http.StatusTooManyRequests, "too many registration attempts")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil || !h.validLoginCSRF(r, r.FormValue("csrf_token")) {
		writePlainError(w, http.StatusForbidden, "invalid registration request")
		return
	}
	// The "website" field is invisible to people; only bots fill it in.
	if r.FormValue("website") != "" {
		writePlainError(w, http.StatusBadRequest, "registration failed")
		return
	}
	lang := languageFromTag(r.FormValue("language"))
	if lang == "" {
		lang = pageLanguage(r)
	}
	clientID := strings.TrimSpace(r.FormValue("client_id"))
	data, ok := h.registrationTarget(r, clientID, safeReturnTo(r.FormValue("return_to")))
	if !ok {
		writePlainError(w, http.StatusForbidden, "registration is not allowed")
		return
	}
	returnTo := data.ReturnTo
	if clientID == "" {
		returnTo = "/account"
	}
	password := []byte(r.FormValue("password"))
	defer auth.ZeroBytes(password)
	user, err := h.service.RegisterPublic(r.Context(), auth.RegisterPublicRequest{
		ClientID: clientID, Email: r.FormValue("email"), Username: r.FormValue("username"), Password: password,
	}, remoteIP(r))
	if err != nil {
		token, tokenErr := auth.NewOpaqueToken("login_csrf_")
		if tokenErr != nil {
			writePlainError(w, http.StatusInternalServerError, "registration failed")
			return
		}
		h.setLoginCSRFCookie(w, token)
		data = withRegistrationLinks(data, lang)
		data.CSRFToken = token
		data.Error = webstatic.T(lang, "register.error_generic")
		if errors.Is(err, auth.ErrEmailExists) || errors.Is(err, auth.ErrUsernameExists) {
			data.Error = webstatic.T(lang, "register.error_taken")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = webstatic.RenderRegistration(w, data)
		return
	}
	session, err := h.service.CreateSessionForUser(r.Context(), user.ID, remoteIP(r), "password")
	if err != nil {
		writePlainError(w, http.StatusInternalServerError, "registration login failed")
		return
	}
	setSessionCookie(w, sessionCookieName, session.SessionToken, session.ExpiresAt, h.secure, true)
	setSessionCookie(w, csrfCookieName, session.CSRFToken, session.ExpiresAt, h.secure, false)
	clearCookie(w, loginCSRFCookieName, h.secure, false)
	redirectAfterForm(w, r, returnTo, lang == webstatic.LangEN)
}

// redirectAfterForm finishes a successful form POST. The CSP's form-action
// 'self' also covers the redirects that follow a form submission, so a POST
// that continues to /oauth/authorize (and from there to the application's
// callback) would be blocked by the browser. Such POSTs hand off through
// /continue, which starts a fresh navigation the policy allows.
func redirectAfterForm(w http.ResponseWriter, r *http.Request, returnTo string, english bool) {
	if !strings.HasPrefix(returnTo, "/oauth/authorize") {
		http.Redirect(w, r, returnTo, http.StatusSeeOther)
		return
	}
	target := "/continue?to=" + url.QueryEscape(returnTo)
	if english {
		target += "&lang=en"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (h *identityAuthHandlers) continuePage(w http.ResponseWriter, r *http.Request) {
	target := safeReturnTo(r.URL.Query().Get("to"))
	if target == "" {
		target = "/account"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = webstatic.RenderContinue(w, webstatic.ContinueData{Lang: pageLanguage(r), Target: target})
}

func (h *identityAuthHandlers) setLoginCSRFCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: loginCSRFCookieName, Value: token, Path: "/",
		MaxAge: int(h.config.LoginCSRFTTL.Seconds()), HttpOnly: false,
		Secure: h.secure, SameSite: http.SameSiteStrictMode,
	})
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
				strings.EqualFold(strings.TrimSpace(r.FormValue("language")), "en"),
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

	redirectAfterForm(
		w,
		r,
		returnTo,
		strings.EqualFold(strings.TrimSpace(r.FormValue("language")), "en"),
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
	applications, err := h.service.ConnectedApplications(r.Context(), result.User.ID)
	if err != nil {
		writePlainError(w, http.StatusInternalServerError, "could not load connected applications")
		return
	}

	_ = webstatic.RenderAccount(
		w,
		webstatic.AccountData{
			Page:                h.page(w, r),
			Email:               result.User.Email,
			DisplayName:         result.User.DisplayName,
			IsSuperadmin:        result.User.IsSuperadmin,
			CSRFToken:           csrfToken,
			AuthMethod:          result.Session.AuthMethod,
			Username:            result.User.Username,
			AccountType:         result.User.AccountType,
			EmailVerifiedAt:     result.User.EmailVerifiedAt,
			Applications:        applications,
			Notice:              r.URL.Query().Get("notice"),
			StrongSetupContinue: strongSetupContinue(r),
		},
	)
}

// strongSetupContinue returns the authorization to resume after the user
// enrolls a strong credential, or "" when no application asked for one.
func strongSetupContinue(r *http.Request) string {
	if r.URL.Query().Get("strong_setup_required") != "1" {
		return ""
	}
	continueTo := safeReturnTo(r.URL.Query().Get("continue_to"))
	if !strings.HasPrefix(continueTo, "/oauth/authorize") {
		return ""
	}
	return continueTo
}

func (h *identityAuthHandlers) removeApplication(w http.ResponseWriter, r *http.Request) {
	_, result, err := h.currentSession(r)
	if err != nil {
		h.clearAuthCookies(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if h.config.StrongAuthMaxAge > 0 && time.Since(result.Session.AuthTime) > h.config.StrongAuthMaxAge {
		http.Redirect(w, r, "/login?auth=strong&return_to=/account", http.StatusSeeOther)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	if err := r.ParseForm(); err != nil {
		writePlainError(w, http.StatusBadRequest, "invalid application removal form")
		return
	}
	csrfCookie := cookieValue(r, csrfCookieName)
	if csrfCookie == "" ||
		!auth.TokensEqual(csrfCookie, r.FormValue("csrf_token")) ||
		!auth.TokenMatchesHash(csrfCookie, result.Session.CSRFHash) ||
		r.FormValue("confirm") != "yes" {
		writePlainError(w, http.StatusForbidden, "invalid application removal request")
		return
	}

	delivery, err := h.service.RemoveConnectedApplication(r.Context(), result.User.ID, r.PathValue("id"), remoteIP(r))
	if errors.Is(err, auth.ErrNotFound) {
		writePlainError(w, http.StatusNotFound, "application is not connected to this account")
		return
	}
	if err != nil {
		writePlainError(w, http.StatusInternalServerError, "application removal failed")
		return
	}
	notice := "app_removed"
	switch delivery.Status {
	case "failed":
		notice = "app_removal_pending"
	case "skipped":
		notice = "app_removed_no_callback"
	}
	http.Redirect(w, r, "/account?notice="+notice, http.StatusSeeOther)
}

func (h *identityAuthHandlers) deleteAccount(w http.ResponseWriter, r *http.Request) {
	_, result, err := h.currentSession(r)
	if err != nil {
		h.clearAuthCookies(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if h.config.StrongAuthMaxAge > 0 && time.Since(result.Session.AuthTime) > h.config.StrongAuthMaxAge {
		writePlainError(w, http.StatusForbidden, "recent authentication required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	if err := r.ParseForm(); err != nil {
		writePlainError(w, http.StatusBadRequest, "invalid account deletion form")
		return
	}
	csrfCookie := cookieValue(r, csrfCookieName)
	if csrfCookie == "" ||
		!auth.TokensEqual(csrfCookie, r.FormValue("csrf_token")) ||
		!auth.TokenMatchesHash(csrfCookie, result.Session.CSRFHash) ||
		r.FormValue("confirmation") != "DELETE" {
		writePlainError(w, http.StatusForbidden, "invalid account deletion request")
		return
	}

	deletion, err := h.service.DeleteAccount(r.Context(), result.User.ID, remoteIP(r))
	if errors.Is(err, auth.ErrAccountDeletionForbidden) {
		writePlainError(w, http.StatusForbidden, "superadmin accounts cannot be self-deleted")
		return
	}
	if err != nil {
		writePlainError(w, http.StatusInternalServerError, "account deletion failed")
		return
	}
	h.clearAuthCookies(w)
	if deletion.Deleted {
		http.Redirect(w, r, "/login?account_deleted=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login?deletion_pending=1", http.StatusSeeOther)
}

func (h *identityAuthHandlers) privacy(w http.ResponseWriter, r *http.Request) {
	h.renderInfo(w, r, "privacy", 5)
}

func (h *identityAuthHandlers) security(w http.ResponseWriter, r *http.Request) {
	h.renderInfo(w, r, "security", 3)
}

// renderInfo renders an information page whose copy lives in the catalog
// under <kind>.title, <kind>.lead and <kind>.s<n>.title/body.
func (h *identityAuthHandlers) renderInfo(w http.ResponseWriter, r *http.Request, kind string, sections int) {
	page := h.page(w, r)
	data := webstatic.InfoData{
		Page:    page,
		Kind:    kind,
		Title:   webstatic.T(page.Lang, kind+".title"),
		Lead:    webstatic.T(page.Lang, kind+".lead"),
		BackURL: "/",
	}
	for i := 1; i <= sections; i++ {
		data.Sections = append(data.Sections, webstatic.InfoSection{
			Title: webstatic.T(page.Lang, fmt.Sprintf("%s.s%d.title", kind, i)),
			Body:  webstatic.T(page.Lang, fmt.Sprintf("%s.s%d.body", kind, i)),
		})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webstatic.RenderInfo(w, data)
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
	english bool,
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

	if english {
		message = "Invalid email or password."
	} else {
		message = "Email ou senha inválidos."
	}
	_ = webstatic.RenderLogin(
		w,
		english,
		webstatic.LoginData{
			CSRFToken: token,
			Error:     message,
			ReturnTo:  returnTo,
		},
	)
}

func loginPageEnglish(r *http.Request) bool {
	return pageLanguage(r) == webstatic.LangEN
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
