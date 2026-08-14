package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
)

type oauthHandlers struct {
	oidc *oidc.Service
	auth *identityAuthHandlers
}

func newOAuthHandlers(
	oidcService *oidc.Service,
	authHandlers *identityAuthHandlers,
) *oauthHandlers {
	return &oauthHandlers{
		oidc: oidcService,
		auth: authHandlers,
	}
}

func (h *oauthHandlers) authorize(
	w http.ResponseWriter,
	r *http.Request,
) {
	request := oidc.AuthorizationRequest{
		ResponseType:        r.URL.Query().Get("response_type"),
		ClientID:            r.URL.Query().Get("client_id"),
		RedirectURI:         r.URL.Query().Get("redirect_uri"),
		Scope:               r.URL.Query().Get("scope"),
		State:               r.URL.Query().Get("state"),
		Nonce:               r.URL.Query().Get("nonce"),
		CodeChallenge:       r.URL.Query().Get("code_challenge"),
		CodeChallengeMethod: r.URL.Query().Get("code_challenge_method"),
	}

	validated, protocolErr := h.oidc.ValidateAuthorization(
		r.Context(),
		request,
	)
	if protocolErr != nil {
		h.authorizationError(w, r, request, protocolErr)
		return
	}

	_, session, err := h.auth.currentSession(r)
	if err != nil {
		loginURL := "/login?return_to=" +
			url.QueryEscape(r.URL.RequestURI())
		http.Redirect(w, r, loginURL, http.StatusSeeOther)
		return
	}

	if protocolErr :=
		h.oidc.AuthorizeIdentityUser(
			r.Context(),
			validated,
			session.User,
		); protocolErr != nil {
		h.authorizationError(
			w,
			r,
			request,
			protocolErr,
		)
		return
	}

	code, err := h.oidc.IssueAuthorizationCode(
		r.Context(),
		validated,
		session.User,
		session.Session.CreatedAt,
		session.Session.AuthMethod,
	)
	if err != nil {
		writePlainError(w, http.StatusInternalServerError, "authorization failed")
		return
	}

	target, err := url.Parse(validated.RedirectURI)
	if err != nil {
		writePlainError(w, http.StatusInternalServerError, "authorization failed")
		return
	}

	query := target.Query()
	query.Set("code", code)
	query.Set("state", validated.State)
	target.RawQuery = query.Encode()

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	http.Redirect(w, r, target.String(), http.StatusFound)
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
