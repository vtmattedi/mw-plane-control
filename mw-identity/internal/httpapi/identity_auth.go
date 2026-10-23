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
	if err := webstatic.RenderLogin(
		w,
		loginPageEnglish(r),
		webstatic.LoginData{
			CSRFToken:       token,
			ReturnTo:        returnTo,
			RegistrationURL: registrationURL,
			RequiredAuth:    requestedAuth,
		},
	); err != nil {
		return
	}
}

func requestedLoginAuth(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
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
	if _, err := h.service.PublicSignupApplication(r.Context(), clientID); err != nil {
		return ""
	}
	values := url.Values{"client_id": {clientID}, "return_to": {returnTo}}
	return "/register?" + values.Encode()
}

func (h *identityAuthHandlers) registerPage(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	returnTo := safeReturnTo(r.URL.Query().Get("return_to"))
	parsedReturn, _ := url.Parse(returnTo)
	app, err := h.service.PublicSignupApplication(r.Context(), clientID)
	if err != nil || parsedReturn.Query().Get("client_id") != clientID || h.registrationURL(r, returnTo) == "" {
		http.NotFound(w, r)
		return
	}
	token, err := auth.NewOpaqueToken("login_csrf_")
	if err != nil {
		writePlainError(w, http.StatusInternalServerError, "could not initialize registration")
		return
	}
	h.setLoginCSRFCookie(w, token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = webstatic.RenderRegistration(w, webstatic.RegistrationData{
		CSRFToken: token, ReturnTo: returnTo, ClientID: clientID, AppName: app.Name,
	})
}

func (h *identityAuthHandlers) register(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Allow(remoteIP(r)) {
		writePlainError(w, http.StatusTooManyRequests, "too many registration attempts")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil || !h.validLoginCSRF(r, r.FormValue("csrf_token")) {
		writePlainError(w, http.StatusForbidden, "invalid registration request")
		return
	}
	clientID := strings.TrimSpace(r.FormValue("client_id"))
	returnTo := safeReturnTo(r.FormValue("return_to"))
	parsedReturn, _ := url.Parse(returnTo)
	app, appErr := h.service.PublicSignupApplication(r.Context(), clientID)
	if appErr != nil || parsedReturn.Query().Get("client_id") != clientID || h.registrationURL(r, returnTo) == "" {
		writePlainError(w, http.StatusForbidden, "registration is not allowed")
		return
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
		message := "Não foi possível criar a conta. Confira os dados informados."
		if errors.Is(err, auth.ErrEmailExists) || errors.Is(err, auth.ErrUsernameExists) {
			message = "O e-mail ou nome de usuário já está em uso."
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = webstatic.RenderRegistration(w, webstatic.RegistrationData{
			CSRFToken: token, Error: message, ReturnTo: returnTo, ClientID: clientID, AppName: app.Name,
		})
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
	h.limiter.Reset(remoteIP(r))
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
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
	applications, err := h.service.ConnectedApplications(r.Context(), result.User.ID)
	if err != nil {
		writePlainError(w, http.StatusInternalServerError, "could not load connected applications")
		return
	}

	_ = webstatic.RenderAccount(
		w,
		webstatic.AccountData{
			Email:           result.User.Email,
			DisplayName:     result.User.DisplayName,
			IsSuperadmin:    result.User.IsSuperadmin,
			CSRFToken:       csrfToken,
			AuthMethod:      result.Session.AuthMethod,
			Username:        result.User.Username,
			AccountType:     result.User.AccountType,
			EmailVerifiedAt: result.User.EmailVerifiedAt,
			Applications:    applications,
			Notice:          r.URL.Query().Get("notice"),
		},
	)
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

func (h *identityAuthHandlers) privacy(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webstatic.RenderInfo(w, webstatic.InfoData{
		Kind: "privacy", Title: "Privacidade", BackURL: "/",
		Lead: "Como o MW Identity usa e protege os dados necessários para autenticar sua conta.",
		Sections: []webstatic.InfoSection{
			{Title: "Dados da conta", Body: "O serviço mantém seu identificador estável, nome de usuário, nome, e-mail, estado de verificação e configurações de autenticação. Aplicativos recebem apenas os dados permitidos pelos escopos autorizados."},
			{Title: "Aplicativos conectados", Body: "A página da conta mostra os aplicativos aos quais sua identidade está conectada. Cada aplicativo mantém separadamente seus próprios perfis e dados de negócio. Ao remover um aplicativo, o MW Identity revoga o acesso dele imediatamente e pede que ele exclua seus dados; o restante da sua conta continua ativo."},
			{Title: "Sessões e auditoria", Body: "Sessões, eventos de segurança e ações administrativas são registrados para proteção, investigação e operação do serviço."},
			{Title: "Credenciais", Body: "Senhas e tokens não são armazenados em texto puro. Segredos sensíveis usam hash ou criptografia apropriada ao seu propósito."},
			{Title: "Exclusão da conta", Body: "Ao excluir sua conta, o MW Identity revoga imediatamente os acessos e notifica os aplicativos conectados. Cada aplicativo exclui ou anonimiza seus dados conforme suas obrigações de retenção. Falhas de entrega mantêm a identidade desativada e a exclusão pendente para nova tentativa administrativa."},
		},
	})
}

func (h *identityAuthHandlers) security(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = webstatic.RenderInfo(w, webstatic.InfoData{
		Kind: "security", Title: "Segurança", BackURL: "/",
		Lead: "Controles disponíveis para manter sua identidade e o ecossistema MattediWorks protegidos.",
		Sections: []webstatic.InfoSection{
			{Title: "Passkeys", Body: "Passkeys são o método principal para acessos administrativos e confirmações de alta segurança."},
			{Title: "Código autenticador", Body: "TOTP funciona como alternativa controlada. Ações críticas ainda podem exigir uma confirmação recente com passkey."},
			{Title: "Recuperação", Body: "A recuperação é iniciada por um operador e usa uma capacidade curta e de uso único para substituir a confiança comprometida."},
		},
	})
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
		message = "E-mail ou senha inválidos."
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
	return r.URL.Path == "/login/en" ||
		strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("lang")), "en")
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
