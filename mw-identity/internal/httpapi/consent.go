package httpapi

import (
	"html/template"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	webstatic "github.com/vtmattedi/mw-plane-control/mw-identity/web/static"
)

// Authorization state stays in bounded, expiring memory, never on disk/logs.
// Restart invalidates unfinished forms; durable consent lives in SQLite.
type pendingAuthorization struct {
	Request   oidc.AuthorizationRequest
	UserID    string
	SessionID string
	Version   int64
	Started   time.Time
	Expires   time.Time
}
type authorizationRequests struct {
	sync.Mutex
	values map[string]pendingAuthorization
}

func (h *oauthHandlers) saveRequest(p pendingAuthorization) (string, error) {
	id, err := auth.NewOpaqueToken("request_")
	if err != nil {
		return "", err
	}
	h.requests.Lock()
	defer h.requests.Unlock()
	if h.requests.values == nil {
		h.requests.values = map[string]pendingAuthorization{}
	}
	for k, v := range h.requests.values {
		if time.Now().After(v.Expires) {
			delete(h.requests.values, k)
		}
	}
	if len(h.requests.values) >= 1024 {
		return "", http.ErrServerClosed
	}
	p.Expires = time.Now().Add(10 * time.Minute)
	h.requests.values[id] = p
	return id, nil
}
func (h *oauthHandlers) loadRequest(id string, consume bool) (pendingAuthorization, bool) {
	h.requests.Lock()
	defer h.requests.Unlock()
	p, ok := h.requests.values[id]
	if consume || time.Now().After(p.Expires) {
		delete(h.requests.values, id)
	}
	return p, ok && time.Now().Before(p.Expires)
}

var consentPage = template.Must(template.New("consent").Parse(`<!doctype html><html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><link rel="stylesheet" href="/static/portal.css"><title>{{.Title}}</title></head><body class="identity-page"><main class="account-shell narrow-shell"><section class="settings-card"><h1>{{.Title}} {{.Name}}</h1><p>{{.Body}}</p><p>{{.Scopes}}</p><form method="post" action="/oauth/consent"><input type="hidden" name="request" value="{{.Handle}}"><input type="hidden" name="csrf_token" value="{{.CSRF}}"><button type="submit" class="primary-button" name="decision" value="allow">{{.Allow}}</button> <button type="submit" class="secondary-button" name="decision" value="cancel">{{.Cancel}}</button></form></section></main></body></html>`))

func (h *oauthHandlers) consent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if r.ParseForm() != nil {
		writePlainError(w, 400, "invalid consent form")
		return
	}
	p, ok := h.loadRequest(r.FormValue("request"), true)
	_, session, err := h.auth.currentSession(r)
	csrf := cookieValue(r, csrfCookieName)
	if !ok || err != nil || p.UserID != session.User.ID || p.SessionID != session.Session.ID || csrf == "" || !auth.TokensEqual(csrf, r.FormValue("csrf_token")) || !auth.TokenMatchesHash(csrf, session.Session.CSRFHash) {
		writePlainError(w, 403, "consent expired; start sign-in again")
		return
	}
	a, e := h.oidc.ValidateAuthorization(r.Context(), p.Request)
	if e != nil {
		h.authorizationError(w, r, p.Request, e)
		return
	}
	if r.FormValue("decision") != "allow" {
		h.authorizationError(w, r, p.Request, &oidc.ProtocolError{Code: "access_denied", Description: "consent declined", HTTPStatus: 403, RedirectAllowed: true})
		return
	}
	if err := h.oidc.GrantConsent(r.Context(), a, session.User, p.Version); err != nil {
		writePlainError(w, 409, "consent changed; start sign-in again")
		return
	}
	a.ConsentVersion = p.Version + 1
	h.finishAuthorization(w, r, a, session.User, session.Session.AuthTime, session.Session.AuthMethod, session.Session.Proof)
}

func (h *oauthHandlers) finishAuthorization(w http.ResponseWriter, r *http.Request, a oidc.ValidatedAuthorization, user auth.User, authTime time.Time, method string, proof string) {
	if e := h.oidc.AuthorizeIdentityUser(r.Context(), a, user); e != nil {
		h.authorizationError(w, r, oidc.AuthorizationRequest{RedirectURI: a.RedirectURI, State: a.State}, e)
		return
	}
	code, err := h.oidc.IssueAuthorizationCode(r.Context(), a, user, authTime, method, proof)
	if err != nil {
		writePlainError(w, 409, "authorization changed; start sign-in again")
		return
	}
	target, _ := url.Parse(a.RedirectURI)
	q := target.Query()
	q.Set("code", code)
	q.Set("state", a.State)
	target.RawQuery = q.Encode()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if r.Method == http.MethodPost {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		lang := "pt"
		if loginPath(r) == "/login/en" {
			lang = "en"
		}
		_ = webstatic.RenderContinue(w, webstatic.ContinueData{Lang: lang, Target: target.String()})
		return
	}
	http.Redirect(w, r, target.String(), http.StatusFound)
}
