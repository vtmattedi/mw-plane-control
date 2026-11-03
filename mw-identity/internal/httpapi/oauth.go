package httpapi

import (
	"encoding/json"

	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitywebauthn"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	webstatic "github.com/vtmattedi/mw-plane-control/mw-identity/web/static"
)

type oauthHandlers struct {
	requests authorizationRequests
	oidc     *oidc.Service
	auth     *identityAuthHandlers
	webauthn *identitywebauthn.Service
	strong   *identitystrong.Service
}

func newOAuthHandlers(
	oidcService *oidc.Service,
	authHandlers *identityAuthHandlers,
	webauthnService *identitywebauthn.Service,
	strongService *identitystrong.Service,
) *oauthHandlers {
	return &oauthHandlers{
		oidc:     oidcService,
		auth:     authHandlers,
		webauthn: webauthnService,
		strong:   strongService,
	}
}

// redirectToStrongSetup sends a user who cannot possibly satisfy a step-up
// challenge (no passkey, no TOTP) to the account page to enroll one, instead
// of the step-up screen, which would otherwise be a dead end. The original
// authorize request is preserved so the account page can offer a way back in.
func (h *oauthHandlers) redirectToStrongSetup(w http.ResponseWriter, r *http.Request) {
	values := url.Values{
		"strong_setup_required": {"1"},
		"continue_to":           {r.URL.RequestURI()},
	}
	http.Redirect(w, r, "/account?"+values.Encode(), http.StatusSeeOther)
}

// hasStrongCredential reports whether the user has any credential capable of
// satisfying a step-up/strong-auth challenge (a passkey or an enrolled TOTP
// authenticator). Users with neither can never complete a step-up prompt, so
// callers must route them to enroll one instead of showing that prompt.
func (h *oauthHandlers) hasStrongCredential(r *http.Request, userID string) bool {
	if h.webauthn == nil || h.strong == nil {
		return false
	}
	credentials, err := h.webauthn.ListCredentials(r.Context(), userID)
	if err == nil && len(credentials) > 0 {
		return true
	}
	totpEnabled, err := h.strong.TOTPStatus(r.Context(), userID)
	return err == nil && totpEnabled
}

func (h *oauthHandlers) authorize(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Cache-Control", "no-store")
	request := oidc.AuthorizationRequest{UILocales: r.URL.Query().Get("ui_locales"), ResponseType: r.URL.Query().Get("response_type"), ClientID: r.URL.Query().Get("client_id"), RedirectURI: r.URL.Query().Get("redirect_uri"), Scope: r.URL.Query().Get("scope"), State: r.URL.Query().Get("state"), Nonce: r.URL.Query().Get("nonce"), CodeChallenge: r.URL.Query().Get("code_challenge"), CodeChallengeMethod: r.URL.Query().Get("code_challenge_method"), ACRValues: r.URL.Query().Get("acr_values"), MaxAge: r.URL.Query().Get("max_age")}
	var previous pendingAuthorization
	resumed := false
	if id := r.URL.Query().Get("request"); id != "" {
		var ok bool
		previous, ok = h.loadRequest(id, false)
		if !ok {
			writePlainError(w, 400, "sign-in expired; start again")
			return
		}
		request = previous.Request
		resumed = true
	}
	q := r.URL.Query()
	q.Set("ui_locales", request.UILocales)
	r.URL.RawQuery = q.Encode()
	validated, e := h.oidc.ValidateAuthorization(r.Context(), request)
	if e != nil {
		h.authorizationError(w, r, request, e)
		return
	}
	_, session, err := h.auth.currentSession(r)
	now := time.Now()
	needsLogin := err != nil || !auth.Satisfies(session.Session.Proof, validated.RequestedACR) || session.Session.AuthTime.After(now)
	mode := "reauth"
	if validated.RequireWebAuthn {
		mode = "webauthn"
		needsLogin = needsLogin || !recentWebAuthnSession(session.Session, time.Duration(max(int64(1), validated.MaxAgeSeconds))*time.Second, now)
	} else if validated.RequestedACR == auth.ACRStrong {
		mode = "strong"
		needsLogin = needsLogin || !strongSessionMethod(session.Session.AuthMethod)
	}
	if err == nil && validated.MaxAgeSet {
		if validated.MaxAgeSeconds == 0 {
			needsLogin = needsLogin || !resumed || previous.SessionID == session.Session.ID || session.Session.AuthTime.Unix() < previous.Started.Unix()
		} else {
			needsLogin = needsLogin || now.Sub(session.Session.AuthTime) > time.Duration(validated.MaxAgeSeconds)*time.Second
		}
	}
	if err != nil && !resumed && !(validated.MaxAgeSet && validated.MaxAgeSeconds == 0) {
		modeQuery := ""
		if mode == "webauthn" || mode == "strong" {
			modeQuery = "&auth=" + mode
		}
		http.Redirect(w, r, loginPath(r)+"?return_to="+url.QueryEscape(r.URL.RequestURI())+modeQuery, http.StatusSeeOther)
		return
	}
	if needsLogin {
		if err == nil && (mode == "webauthn" || mode == "strong") && !h.hasStrongCredential(r, session.User.ID) {
			h.redirectToStrongSetup(w, r)
			return
		}
		started := now
		if resumed {
			started = previous.Started
		}
		id, err := h.saveRequest(pendingAuthorization{Request: request, SessionID: session.Session.ID, Started: started})
		if err != nil {
			writePlainError(w, 429, "too many sign-in requests")
			return
		}
		http.Redirect(w, r, loginPath(r)+"?return_to="+url.QueryEscape("/oauth/authorize?request="+id)+"&auth="+mode, http.StatusSeeOther)
		return
	}
	if e := h.oidc.AuthorizeIdentityUser(r.Context(), validated, session.User); e != nil {
		h.authorizationError(w, r, request, e)
		return
	}
	c, err := h.oidc.Consent(r.Context(), validated, session.User)
	if err != nil {
		writePlainError(w, 500, "could not load consent")
		return
	}
	if !oidc.ConsentCovers(c, validated.Scopes) {
		csrf := cookieValue(r, csrfCookieName)
		if csrf == "" || !auth.TokenMatchesHash(csrf, session.Session.CSRFHash) {
			writePlainError(w, 403, "session expired; sign in again")
			return
		}
		id, err := h.saveRequest(pendingAuthorization{Request: request, UserID: session.User.ID, SessionID: session.Session.ID, Version: c.Version, Started: now})
		if err != nil {
			writePlainError(w, 429, "too many requests")
			return
		}
		data := map[string]string{"Lang": "pt", "Title": "Continuar com MW em", "Body": "Compartilhar os campos de identidade abaixo. O aplicativo gerencia sua conta, dados e sessões.", "Allow": "Continuar", "Cancel": "Cancelar", "Name": validated.Application.Name, "Scopes": strings.Join(validated.Scopes, ", "), "Handle": id, "CSRF": csrf}
		if loginPath(r) == "/login/en" {
			data["Lang"] = "en"
			data["Title"] = "Continue with MW to"
			data["Body"] = "Share the identity fields below. The application manages your account, data and sessions."
			data["Allow"] = "Continue"
			data["Cancel"] = "Cancel"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = consentPage.Execute(w, data)
		return
	}
	validated.ConsentVersion = c.Version
	if id := r.URL.Query().Get("request"); id != "" {
		h.loadRequest(id, true)
	}
	h.finishAuthorization(w, r, validated, session.User, session.Session.AuthTime, session.Session.AuthMethod, session.Session.Proof)
}

func (h *oauthHandlers) token(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	if !strings.HasPrefix(
		strings.ToLower(r.Header.Get("Content-Type")),
		"application/x-www-form-urlencoded",
	) {
		writeOAuthError(w, &oidc.ProtocolError{
			Code:        "invalid_request",
			Description: "content type must be application/x-www-form-urlencoded",
			HTTPStatus:  http.StatusBadRequest,
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, &oidc.ProtocolError{
			Code:        "invalid_request",
			Description: "invalid token request form",
			HTTPStatus:  http.StatusBadRequest,
		})
		return
	}

	clientID, clientSecret, protocolErr := clientCredentials(r)
	if protocolErr != nil {
		writeOAuthError(w, protocolErr)
		return
	}

	var response oidc.TokenResponse
	switch r.FormValue("grant_type") {
	case "authorization_code":
		response, protocolErr = h.oidc.ExchangeCode(
			r.Context(),
			oidc.TokenRequest{
				GrantType:    r.FormValue("grant_type"),
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Code:         r.FormValue("code"),
				RedirectURI:  r.FormValue("redirect_uri"),
				CodeVerifier: r.FormValue("code_verifier"),
			},
		)
	case oidc.TokenExchangeGrantType:
		response, protocolErr = h.oidc.ExchangeToken(
			r.Context(),
			oidc.TokenExchangeRequest{
				GrantType:          r.FormValue("grant_type"),
				ClientID:           clientID,
				ClientSecret:       clientSecret,
				SubjectToken:       r.FormValue("subject_token"),
				SubjectTokenType:   r.FormValue("subject_token_type"),
				RequestedTokenType: r.FormValue("requested_token_type"),
				Audience:           r.FormValue("audience"),
				Scope:              r.FormValue("scope"),
			},
		)
	default:
		protocolErr = oidc.UnsupportedGrantType()
	}
	if protocolErr != nil {
		writeOAuthError(w, protocolErr)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *oauthHandlers) introspect(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
		writeOAuthError(w, &oidc.ProtocolError{Code: "invalid_request", Description: "content type must be application/x-www-form-urlencoded", HTTPStatus: http.StatusBadRequest})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, &oidc.ProtocolError{Code: "invalid_request", Description: "invalid introspection form", HTTPStatus: http.StatusBadRequest})
		return
	}
	clientID, clientSecret, protocolErr := clientCredentials(r)
	if protocolErr != nil {
		writeOAuthError(w, protocolErr)
		return
	}
	response, protocolErr := h.oidc.Introspect(r.Context(), clientID, clientSecret, r.FormValue("token"))
	if protocolErr != nil {
		writeOAuthError(w, protocolErr)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *oauthHandlers) userInfo(
	w http.ResponseWriter,
	r *http.Request,
) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	claims, protocolErr := h.oidc.UserInfo(
		r.Context(),
		bearerToken(r),
	)
	if protocolErr != nil {
		w.Header().Set(
			"WWW-Authenticate",
			`Bearer error="invalid_token"`,
		)
		writeOAuthError(w, protocolErr)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(claims)
}

func (h *oauthHandlers) authorizationError(
	w http.ResponseWriter,
	r *http.Request,
	request oidc.AuthorizationRequest,
	protocolErr *oidc.ProtocolError,
) {
	if !protocolErr.RedirectAllowed {
		writeOAuthError(w, protocolErr)
		return
	}

	target, err := url.Parse(request.RedirectURI)
	if err != nil {
		writeOAuthError(w, protocolErr)
		return
	}

	query := target.Query()
	query.Set("error", protocolErr.Code)
	query.Set("error_description", protocolErr.Description)
	if request.State != "" {
		query.Set("state", request.State)
	}
	target.RawQuery = query.Encode()

	if r.Method == http.MethodPost {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = webstatic.RenderContinue(w, webstatic.ContinueData{Lang: pageLanguage(r), Target: target.String()})
		return
	}
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func clientCredentials(
	r *http.Request,
) (string, string, *oidc.ProtocolError) {
	basicID, basicSecret, hasBasic := r.BasicAuth()
	formID := strings.TrimSpace(r.FormValue("client_id"))
	formSecret := r.FormValue("client_secret")

	if hasBasic {
		if formID != "" && formID != basicID {
			return "", "", &oidc.ProtocolError{
				Code:        "invalid_request",
				Description: "conflicting client_id values",
				HTTPStatus:  http.StatusBadRequest,
			}
		}
		return basicID, basicSecret, nil
	}

	if formID == "" {
		return "", "", &oidc.ProtocolError{
			Code:        "invalid_client",
			Description: "client authentication failed",
			HTTPStatus:  http.StatusUnauthorized,
		}
	}

	return formID, formSecret, nil
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "

	if len(value) <= len(prefix) ||
		!strings.EqualFold(value[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(value[len(prefix):])
}

func writeOAuthError(
	w http.ResponseWriter,
	protocolErr *oidc.ProtocolError,
) {
	status := protocolErr.HTTPStatus
	if status == 0 {
		status = http.StatusBadRequest
	}

	if protocolErr.Code == "invalid_client" {
		w.Header().Set(
			"WWW-Authenticate",
			`Basic realm="mw-identity-token"`,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             protocolErr.Code,
		"error_description": protocolErr.Description,
	})
}
