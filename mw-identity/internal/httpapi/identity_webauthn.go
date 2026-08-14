package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitywebauthn"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/portal"
)

const (
	identityWebAuthnCeremonyHeader = "X-MW-WebAuthn-Ceremony"
	identityWebAuthnCSRFHeader     = "X-MW-CSRF"
	maxIdentityWebAuthnBody        = 1 << 20
)

type identityWebAuthnHandlers struct {
	service      *identitywebauthn.Service
	auth         *identityAuthHandlers
	strongMaxAge time.Duration
}

type identityWebAuthnOptionsResponse struct {
	CeremonyToken string `json:"ceremony_token"`
	Options       any    `json:"options"`
}

func newIdentityWebAuthnHandlers(
	service *identitywebauthn.Service,
	authHandlers *identityAuthHandlers,
	strongMaxAge time.Duration,
) *identityWebAuthnHandlers {
	return &identityWebAuthnHandlers{service: service, auth: authHandlers, strongMaxAge: strongMaxAge}
}

func (h *identityWebAuthnHandlers) loginScript(
	w http.ResponseWriter,
	_ *http.Request,
) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(portal.IdentityWebAuthnLoginJS))
}

func (h *identityWebAuthnHandlers) accountScript(
	w http.ResponseWriter,
	_ *http.Request,
) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(portal.IdentityWebAuthnAccountJS))
}

func (h *identityWebAuthnHandlers) list(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, session, err := h.auth.currentSession(r)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	items, err := h.service.ListCredentials(
		r.Context(),
		session.User.ID,
	)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "webauthn_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"credentials": items})
}

func (h *identityWebAuthnHandlers) registerOptions(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, session, ok := h.sessionAndCSRF(w, r)
	if !ok {
		return
	}
	existingCredentials, err := h.service.ListCredentials(r.Context(), session.User.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "webauthn_failed")
		return
	}
	if len(existingCredentials) > 0 && !recentWebAuthnSession(session.Session, h.strongMaxAge, time.Now()) {
		writeStepUpRequired(w)
		return
	}

	var request struct {
		Label string `json:"label"`
	}
	if !decodeJSONBody(w, r, &request) {
		return
	}

	options, token, err := h.service.BeginRegistration(
		r.Context(),
		session.User,
		request.Label,
	)
	if err != nil {
		status := http.StatusInternalServerError
		code := "webauthn_failed"
		reason := "begin_registration_failed"
		if errors.Is(err, identitywebauthn.ErrInvalidLabel) {
			status = http.StatusBadRequest
			code = "invalid_label"
			reason = "invalid_label"
		}
		h.service.RecordAudit(
			r.Context(), session.User.ID, remoteIP(r),
			"identity.webauthn.registration.failed", "failure", reason,
		)
		writeJSONError(w, status, code)
		return
	}

	h.service.RecordAudit(
		r.Context(), session.User.ID, remoteIP(r),
		"identity.webauthn.registration.started", "success", "started",
	)
	writeJSON(w, http.StatusOK, identityWebAuthnOptionsResponse{
		CeremonyToken: token,
		Options:       options,
	})
}

func (h *identityWebAuthnHandlers) registerVerify(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, session, ok := h.sessionAndCSRF(w, r)
	if !ok {
		return
	}
	existingCredentials, err := h.service.ListCredentials(r.Context(), session.User.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "webauthn_failed")
		return
	}
	if len(existingCredentials) > 0 && !recentWebAuthnSession(session.Session, h.strongMaxAge, time.Now()) {
		writeStepUpRequired(w)
		return
	}

	ceremony := strings.TrimSpace(
		r.Header.Get(identityWebAuthnCeremonyHeader),
	)
	if ceremony == "" {
		writeJSONError(w, http.StatusBadRequest, "webauthn_ceremony_required")
		return
	}
	if !isJSONRequest(r) {
		writeJSONError(w, http.StatusUnsupportedMediaType, "json_required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxIdentityWebAuthnBody)
	item, err := h.service.FinishRegistration(
		r.Context(),
		session.User.ID,
		ceremony,
		r,
	)
	if err != nil {
		status, reason := identityWebAuthnFailure(err)
		h.service.RecordAudit(
			r.Context(), session.User.ID, remoteIP(r),
			"identity.webauthn.registration.failed", "failure", reason,
		)
		writeJSONError(w, status, "webauthn_verification_failed")
		return
	}

	h.service.RecordAudit(
		r.Context(), session.User.ID, remoteIP(r),
		"identity.webauthn.credential.registered", "success", "registered",
	)
	writeJSON(w, http.StatusCreated, item)
}

func (h *identityWebAuthnHandlers) loginOptions(
	w http.ResponseWriter,
	r *http.Request,
) {
	if !h.auth.limiter.Allow(remoteIP(r)) {
		writeJSONError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}

	options, token, err := h.service.BeginLogin(r.Context())
	if err != nil {
		h.service.RecordAudit(
			r.Context(), "", remoteIP(r),
			"identity.webauthn.login.failed", "failure", "begin_failed",
		)
		writeJSONError(w, http.StatusInternalServerError, "webauthn_failed")
		return
	}

	writeJSON(w, http.StatusOK, identityWebAuthnOptionsResponse{
		CeremonyToken: token,
		Options:       options,
	})
}

func (h *identityWebAuthnHandlers) loginVerify(
	w http.ResponseWriter,
	r *http.Request,
) {
	ceremony := strings.TrimSpace(
		r.Header.Get(identityWebAuthnCeremonyHeader),
	)
	if ceremony == "" {
		writeJSONError(w, http.StatusBadRequest, "webauthn_ceremony_required")
		return
	}
	if !isJSONRequest(r) {
		writeJSONError(w, http.StatusUnsupportedMediaType, "json_required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxIdentityWebAuthnBody)
	user, err := h.service.FinishLogin(
		r.Context(),
		ceremony,
		r,
	)
	if err != nil {
		status, reason := identityWebAuthnFailure(err)
		if errors.Is(err, identitywebauthn.ErrUserDisabled) {
			status = http.StatusUnauthorized
			reason = "user_disabled"
		}
		h.service.RecordAudit(
			r.Context(), "", remoteIP(r),
			"identity.webauthn.login.failed", "failure", reason,
		)
		writeJSONError(w, status, "webauthn_login_failed")
		return
	}

	result, err := h.auth.service.CreateSessionForUser(
		r.Context(),
		user.ID,
		remoteIP(r),
		"webauthn",
	)
	if err != nil {
		h.service.RecordAudit(
			r.Context(), user.ID, remoteIP(r),
			"identity.webauthn.login.failed", "failure", "session_failed",
		)
		writeJSONError(w, http.StatusInternalServerError, "session_failed")
		return
	}

	setSessionCookie(
		w,
		sessionCookieName,
		result.SessionToken,
		result.ExpiresAt,
		h.auth.secure,
		true,
	)
	setSessionCookie(
		w,
		csrfCookieName,
		result.CSRFToken,
		result.ExpiresAt,
		h.auth.secure,
		false,
	)
	clearCookie(w, loginCSRFCookieName, h.auth.secure, false)
	h.auth.limiter.Reset(remoteIP(r))

	h.service.RecordAudit(
		r.Context(), user.ID, remoteIP(r),
		"identity.webauthn.login.completed", "success", "authenticated",
	)

	redirectTo := safeReturnTo(r.URL.Query().Get("return_to"))
	if redirectTo == "" {
		redirectTo = "/account"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"redirect_to": redirectTo,
		"csrf_token":  result.CSRFToken,
		"expires_at":  result.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *identityWebAuthnHandlers) remove(
	w http.ResponseWriter,
	r *http.Request,
) {
	_, session, ok := h.sessionAndCSRF(w, r)
	if !ok {
		return
	}

	items, listErr := h.service.ListCredentials(r.Context(), session.User.ID)
	if listErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "webauthn_failed")
		return
	}
	if len(items) <= 1 {
		writeJSONError(w, http.StatusConflict, "cannot_remove_last_passkey")
		return
	}
	if !recentWebAuthnSession(session.Session, h.strongMaxAge, time.Now()) {
		writeStepUpRequired(w)
		return
	}

	err := h.service.RevokeCredential(
		r.Context(),
		session.User.ID,
		r.PathValue("credential_id"),
	)
	if err != nil {
		if errors.Is(err, identitywebauthn.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "credential_not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "webauthn_failed")
		return
	}

	h.service.RecordAudit(
		r.Context(), session.User.ID, remoteIP(r),
		"identity.webauthn.credential.removed", "success", "revoked",
	)
	w.WriteHeader(http.StatusNoContent)
}

func (h *identityWebAuthnHandlers) sessionAndCSRF(
	w http.ResponseWriter,
	r *http.Request,
) (string, auth.SessionResult, bool) {
	raw, session, err := h.auth.currentSession(r)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return "", auth.SessionResult{}, false
	}

	cookieToken := cookieValue(r, csrfCookieName)
	headerToken := strings.TrimSpace(r.Header.Get(identityWebAuthnCSRFHeader))
	if cookieToken == "" ||
		headerToken == "" ||
		!auth.TokensEqual(cookieToken, headerToken) ||
		!auth.TokenMatchesHash(cookieToken, session.Session.CSRFHash) {
		writeJSONError(w, http.StatusForbidden, "csrf_failed")
		return "", auth.SessionResult{}, false
	}

	return raw, session, true
}

func identityWebAuthnFailure(err error) (int, string) {
	switch {
	case errors.Is(err, identitywebauthn.ErrCeremonyExpired):
		return http.StatusBadRequest, "challenge_expired"
	case errors.Is(err, identitywebauthn.ErrCeremonyUsed):
		return http.StatusBadRequest, "challenge_replayed"
	case errors.Is(err, identitywebauthn.ErrInvalidCeremony):
		return http.StatusBadRequest, "challenge_invalid"
	case errors.Is(err, identitywebauthn.ErrConflict):
		return http.StatusConflict, "credential_conflict"
	case errors.Is(err, identitywebauthn.ErrVerification):
		return http.StatusUnauthorized, "verification_failed"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

func isJSONRequest(r *http.Request) bool {
	return strings.HasPrefix(
		strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type"))),
		"application/json",
	)
}
