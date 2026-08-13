package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookieName = "linkrp_session"
	pendingCookieName = "linkrp_pending"
)

type App struct {
	cfg   Config
	state *State
	oidc  *OIDCClient
}

func NewApp(cfg Config, state *State, oidc *OIDCClient) *App {
	return &App{cfg: cfg, state: state, oidc: oidc}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("POST /local/login", a.localLogin)
	mux.HandleFunc("POST /local/logout", a.localLogout)
	mux.HandleFunc("GET /me", a.me)
	mux.HandleFunc("GET /link/start", a.linkStart)
	mux.HandleFunc("GET /auth/mw/start", a.signInStart)
	mux.HandleFunc("GET /oidc/callback", a.callback)
	mux.HandleFunc("GET /link-required", a.linkRequired)
	mux.HandleFunc("POST /link-existing", a.linkExisting)
	mux.HandleFunc("POST /unlink", a.unlink)
	mux.HandleFunc("GET /test/audit", a.testAudit)
	return a.securityHeaders(mux)
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/oidc/") || strings.HasPrefix(r.URL.Path, "/link-") || strings.HasPrefix(r.URL.Path, "/auth/") {
			w.Header().Set("Cache-Control", "no-store, max-age=0")
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "account-link-rp"})
}

func (a *App) localLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_form")
		return
	}
	user, ok := a.state.AuthenticateLocal(r.FormValue("email"), r.FormValue("password"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_local_credentials")
		return
	}
	raw, err := a.state.CreateSession(user.ID, "local_password")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_failed")
		return
	}
	a.setSessionCookie(w, raw)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) localLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.state.DeleteSession(cookie.Value)
	}
	a.clearCookie(w, sessionCookieName)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	session, user, ok := a.currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not_authenticated")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"local_user":          user,
		"auth_method":         session.AuthMethod,
		"external_identities": a.state.IdentitiesForUser(user.ID),
	})
}

func (a *App) linkStart(w http.ResponseWriter, r *http.Request) {
	session, user, ok := a.currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "local_auth_required")
		return
	}
	_, state, nonce, err := a.state.BeginTransaction("link", user.ID, session.TokenHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transaction_failed")
		return
	}
	transaction, err := a.state.transactionsSnapshot(hashOpaque(state))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transaction_failed")
		return
	}
	http.Redirect(w, r, a.oidc.AuthorizationURL(state, nonce, transaction.Verifier), http.StatusFound)
}

func (a *App) signInStart(w http.ResponseWriter, r *http.Request) {
	_, state, nonce, err := a.state.BeginTransaction("signin", "", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transaction_failed")
		return
	}
	transaction, err := a.state.transactionsSnapshot(hashOpaque(state))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transaction_failed")
		return
	}
	http.Redirect(w, r, a.oidc.AuthorizationURL(state, nonce, transaction.Verifier), http.StatusFound)
}

func (a *App) callback(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	stateRaw := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || stateRaw == "" {
		writeError(w, http.StatusBadRequest, "oidc_callback_invalid")
		return
	}
	transaction, err := a.state.ClaimTransaction(stateRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "oidc_transaction_invalid")
		return
	}
	identity, err := a.oidc.ExchangeAndValidate(r.Context(), code, transaction.Verifier, transaction.NonceHash)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oidc_validation_failed")
		return
	}

	switch transaction.Mode {
	case "link":
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "local_auth_required")
			return
		}
		user, err := a.state.AssertLinkSession(cookie.Value, transaction)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "link_session_changed")
			return
		}
		linked, err := a.state.LinkIdentity(user.ID, identity)
		if errors.Is(err, ErrDuplicateSubject) || errors.Is(err, ErrIssuerAlreadyLink) {
			a.state.Audit(AuditEvent{Type: "link.duplicate_rejected", LocalUserID: user.ID, Issuer: identity.Issuer, Subject: identity.Subject})
			writeError(w, http.StatusConflict, "identity_already_linked")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "link_failed")
			return
		}
		a.state.Audit(AuditEvent{Type: "link.completed", LocalUserID: linked.LocalUserID, Issuer: linked.Issuer, Subject: linked.Subject})
		http.Redirect(w, r, "/me", http.StatusSeeOther)

	case "signin":
		if linked, ok := a.state.Identity(identity.Issuer, identity.Subject); ok {
			a.state.MarkIdentityLogin(identity.Issuer, identity.Subject)
			raw, err := a.state.CreateSession(linked.LocalUserID, "mw_identity")
			if err != nil {
				writeError(w, http.StatusInternalServerError, "session_failed")
				return
			}
			a.setSessionCookie(w, raw)
			a.state.Audit(AuditEvent{Type: "signin.completed", LocalUserID: linked.LocalUserID, Issuer: identity.Issuer, Subject: identity.Subject})
			http.Redirect(w, r, "/me", http.StatusSeeOther)
			return
		}
		pendingRaw, err := a.state.CreatePending(identity)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "pending_link_failed")
			return
		}
		a.setPendingCookie(w, pendingRaw)
		a.state.Audit(AuditEvent{Type: "link.authentication_required", Issuer: identity.Issuer, Subject: identity.Subject})
		http.Redirect(w, r, "/link-required", http.StatusSeeOther)

	default:
		writeError(w, http.StatusBadRequest, "transaction_mode_invalid")
	}
}

func (a *App) linkRequired(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(pendingCookieName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "pending_link_missing")
		return
	}
	pending, err := a.state.Pending(cookie.Value)
	if err != nil {
		writeError(w, http.StatusBadRequest, "pending_link_invalid")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"requires_existing_auth": true,
		"identity_email":         pending.Identity.Email,
		"issuer":                 pending.Identity.Issuer,
		"subject_present":        pending.Identity.Subject != "",
	})
}

func (a *App) linkExisting(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(pendingCookieName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "pending_link_missing")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_form")
		return
	}
	user, ok := a.state.AuthenticateLocal(r.FormValue("email"), r.FormValue("password"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_local_credentials")
		return
	}
	linked, err := a.state.CompletePending(cookie.Value, user.ID)
	if errors.Is(err, ErrDuplicateSubject) || errors.Is(err, ErrIssuerAlreadyLink) {
		writeError(w, http.StatusConflict, "identity_already_linked")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "pending_link_invalid")
		return
	}
	raw, err := a.state.CreateSession(user.ID, "mw_identity_linked_existing")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_failed")
		return
	}
	a.setSessionCookie(w, raw)
	a.clearCookie(w, pendingCookieName)
	a.state.Audit(AuditEvent{Type: "link.existing_completed", LocalUserID: user.ID, Issuer: linked.Issuer, Subject: linked.Subject})
	http.Redirect(w, r, "/me", http.StatusSeeOther)
}

func (a *App) unlink(w http.ResponseWriter, r *http.Request) {
	_, user, ok := a.currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not_authenticated")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_form")
		return
	}
	if _, ok := a.state.AuthenticateLocalUser(user.ID, r.FormValue("password")); !ok {
		writeError(w, http.StatusForbidden, "fresh_local_auth_required")
		return
	}
	unlinked, err := a.state.UnlinkIdentity(user.ID, a.cfg.IdentityIssuer)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "identity_link_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unlink_failed")
		return
	}
	// This proof harness revokes all app sessions after unlink. Real apps may
	// choose an equivalent policy, but must never leave an Identity-only user
	// locked out after self-service unlink.
	a.state.DeleteSessionsForUser(user.ID)
	a.clearCookie(w, sessionCookieName)
	a.state.Audit(AuditEvent{Type: "unlink.completed", LocalUserID: user.ID, Issuer: unlinked.Issuer, Subject: unlinked.Subject})
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) testAudit(w http.ResponseWriter, _ *http.Request) {
	if !a.cfg.TestMode {
		http.NotFound(w, nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": a.state.AuditEvents()})
}

func (a *App) currentSession(r *http.Request) (Session, LocalUser, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return Session{}, LocalUser{}, false
	}
	session, user, err := a.state.Session(cookie.Value)
	return session, user, err == nil
}

func (a *App) setSessionCookie(w http.ResponseWriter, raw string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: raw, Path: "/", HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: int(a.cfg.SessionTTL.Seconds())})
}

func (a *App) setPendingCookie(w http.ResponseWriter, raw string) {
	http.SetCookie(w, &http.Cookie{Name: pendingCookieName, Value: raw, Path: "/", HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: int(a.cfg.PendingTTL.Seconds())})
}

func (a *App) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

// transactionsSnapshot returns only a transaction already stored by BeginTransaction.
// It exists so the HTTP layer never needs to create a second verifier.
func (s *State) transactionsSnapshot(stateHash string) (OIDCTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.transactions[stateHash]
	if !ok {
		return OIDCTransaction{}, fmt.Errorf("transaction missing")
	}
	return value, nil
}
