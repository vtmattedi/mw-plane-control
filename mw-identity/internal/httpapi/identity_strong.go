package httpapi

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitywebauthn"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/portal"
	webstatic "github.com/vtmattedi/mw-plane-control/mw-identity/web/static"
)

const identityRecoveryHeader = "X-MW-Recovery"

const totpQRCodeSize = 256

type totpSetupResponse struct {
	Secret        string `json:"secret"`
	OTPAuthURI    string `json:"otpauth_uri"`
	QRCodeDataURL string `json:"qr_code_data_url"`
}

type identityStrongHandlers struct {
	service      *identitystrong.Service
	webauthn     *identitywebauthn.Service
	auth         *identityAuthHandlers
	strongMaxAge time.Duration
}

func newIdentityStrongHandlers(service *identitystrong.Service, webauthn *identitywebauthn.Service, authHandlers *identityAuthHandlers, strongMaxAge time.Duration) *identityStrongHandlers {
	return &identityStrongHandlers{service: service, webauthn: webauthn, auth: authHandlers, strongMaxAge: strongMaxAge}
}

func (h *identityStrongHandlers) status(w http.ResponseWriter, r *http.Request) {
	_, session, err := h.auth.currentSession(r)
	if err != nil {
		writeJSONError(w, 401, "unauthorized")
		return
	}
	enabled, err := h.service.TOTPStatus(r.Context(), session.User.ID)
	if err != nil {
		writeJSONError(w, 500, "strong_auth_failed")
		return
	}
	writeJSON(w, 200, map[string]any{"totp_enabled": enabled, "auth_method": session.Session.AuthMethod, "auth_time": session.Session.AuthTime.Unix(), "recent_webauthn": recentWebAuthnSession(session.Session, h.strongMaxAge, time.Now())})
}

func (h *identityStrongHandlers) setupTOTP(w http.ResponseWriter, r *http.Request) {
	_, session, ok := h.sessionCSRFStrong(w, r)
	if !ok {
		return
	}
	if !recentWebAuthnSession(session.Session, h.strongMaxAge, time.Now()) {
		writeStepUpRequired(w)
		return
	}
	result, err := h.service.SetupTOTP(r.Context(), session.User)
	if err != nil {
		if errors.Is(err, identitystrong.ErrTOTPAlreadyEnabled) {
			writeJSONError(w, 409, "totp_already_enabled")
			return
		}
		writeJSONError(w, 500, "totp_setup_failed")
		return
	}
	payload, err := newTOTPSetupResponse(result)
	if err != nil {
		writeJSONError(w, 500, "totp_setup_failed")
		return
	}
	writeJSON(w, 200, payload)
}

func newTOTPSetupResponse(result identitystrong.TOTPSetup) (totpSetupResponse, error) {
	png, err := qrcode.Encode(result.OTPAuthURI, qrcode.Medium, totpQRCodeSize)
	if err != nil {
		return totpSetupResponse{}, err
	}
	return totpSetupResponse{
		Secret:        result.Secret,
		OTPAuthURI:    result.OTPAuthURI,
		QRCodeDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	}, nil
}

func (h *identityStrongHandlers) confirmTOTP(w http.ResponseWriter, r *http.Request) {
	_, session, ok := h.sessionCSRFStrong(w, r)
	if !ok {
		return
	}
	if !recentWebAuthnSession(session.Session, h.strongMaxAge, time.Now()) {
		writeStepUpRequired(w)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.service.ConfirmTOTP(r.Context(), session.User.ID, req.Code); err != nil {
		status := 400
		if !errors.Is(err, identitystrong.ErrInvalidCode) {
			status = 500
		}
		writeJSONError(w, status, "totp_confirm_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *identityStrongHandlers) disableTOTP(w http.ResponseWriter, r *http.Request) {
	_, session, ok := h.sessionCSRFStrong(w, r)
	if !ok {
		return
	}
	if !recentWebAuthnSession(session.Session, h.strongMaxAge, time.Now()) {
		writeStepUpRequired(w)
		return
	}
	if err := h.service.DisableTOTP(r.Context(), session.User.ID); err != nil {
		writeJSONError(w, 500, "totp_disable_failed")
		return
	}
	w.WriteHeader(204)
}

func (h *identityStrongHandlers) loginTOTP(w http.ResponseWriter, r *http.Request) {
	if !h.auth.limiter.Allow(remoteIP(r)) {
		writeJSONError(w, 429, "rate_limited")
		return
	}
	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		ReturnTo string `json:"return_to"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	user, err := h.service.AuthenticateTOTP(r.Context(), req.Email, req.Code)
	if err != nil {
		writeJSONError(w, 401, "totp_login_failed")
		return
	}
	result, err := h.auth.service.CreateSessionForUser(r.Context(), user.ID, remoteIP(r), "totp")
	if err != nil {
		writeJSONError(w, 500, "session_failed")
		return
	}
	setSessionCookie(w, sessionCookieName, result.SessionToken, result.ExpiresAt, h.auth.secure, true)
	setSessionCookie(w, csrfCookieName, result.CSRFToken, result.ExpiresAt, h.auth.secure, false)
	clearCookie(w, loginCSRFCookieName, h.auth.secure, false)
	h.auth.limiter.Reset(remoteIP(r))
	target := safeReturnTo(req.ReturnTo)
	if target == "" {
		target = "/account"
	}
	writeJSON(w, 200, map[string]any{"redirect_to": target})
}

func (h *identityStrongHandlers) recoveryPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = webstatic.RenderRecovery(w)
}
func (h *identityStrongHandlers) recoveryScript(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(portal.IdentityRecoveryJS))
}

func (h *identityStrongHandlers) recoveryOptions(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.Header.Get(identityRecoveryHeader))
	rec, user, err := h.service.Recovery(r.Context(), raw)
	_ = rec
	if err != nil {
		writeJSONError(w, 401, "invalid_recovery")
		return
	}
	options, ceremony, err := h.webauthn.BeginRegistration(r.Context(), user, "Recovery passkey")
	if err != nil {
		writeJSONError(w, 500, "recovery_failed")
		return
	}
	writeJSON(w, 200, identityWebAuthnOptionsResponse{CeremonyToken: ceremony, Options: options})
}

func (h *identityStrongHandlers) recoveryVerify(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.Header.Get(identityRecoveryHeader))
	rec, user, err := h.service.Recovery(r.Context(), raw)
	if err != nil {
		writeJSONError(w, 401, "invalid_recovery")
		return
	}
	ceremony := strings.TrimSpace(r.Header.Get(identityWebAuthnCeremonyHeader))
	if ceremony == "" {
		writeJSONError(w, 400, "webauthn_ceremony_required")
		return
	}
	if !isJSONRequest(r) {
		writeJSONError(w, 415, "json_required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxIdentityWebAuthnBody)
	credential, err := h.webauthn.FinishRegistration(r.Context(), user.ID, ceremony, r)
	if err != nil {
		writeJSONError(w, 401, "recovery_webauthn_failed")
		return
	}
	if err := h.webauthn.RevokeOtherCredentials(r.Context(), user.ID, credential.CredentialID); err != nil {
		writeJSONError(w, 500, "recovery_failed")
		return
	}
	if err := h.service.CompleteRecovery(r.Context(), rec.ID, user.ID); err != nil {
		writeJSONError(w, 500, "recovery_failed")
		return
	}
	result, err := h.auth.service.CreateSessionForUser(r.Context(), user.ID, remoteIP(r), "recovery")
	if err != nil {
		writeJSONError(w, 500, "session_failed")
		return
	}
	setSessionCookie(w, sessionCookieName, result.SessionToken, result.ExpiresAt, h.auth.secure, true)
	setSessionCookie(w, csrfCookieName, result.CSRFToken, result.ExpiresAt, h.auth.secure, false)
	writeJSON(w, 200, map[string]any{"redirect_to": "/account"})
}

func (h *identityStrongHandlers) sessionCSRFStrong(w http.ResponseWriter, r *http.Request) (string, auth.SessionResult, bool) {
	raw, session, err := h.auth.currentSession(r)
	if err != nil {
		writeJSONError(w, 401, "unauthorized")
		return "", auth.SessionResult{}, false
	}
	cookie := cookieValue(r, csrfCookieName)
	header := strings.TrimSpace(r.Header.Get(identityWebAuthnCSRFHeader))
	if cookie == "" || header == "" || !auth.TokensEqual(cookie, header) || !auth.TokenMatchesHash(cookie, session.Session.CSRFHash) {
		writeJSONError(w, 403, "csrf_failed")
		return "", auth.SessionResult{}, false
	}
	return raw, session, true
}
