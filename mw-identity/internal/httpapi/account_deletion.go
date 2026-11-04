package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

func (h *identityAuthHandlers) deleteAccount(w http.ResponseWriter, r *http.Request) {
	_, result, err := h.currentSession(r)
	if err != nil {
		h.clearAuthCookies(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
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

	if !h.recentAccountAuthentication(result.Session) {
		http.Redirect(w, r, accountReauthenticationURL, http.StatusSeeOther)
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
	if deletion.StatusToken != "" {
		http.SetCookie(w, &http.Cookie{Name: deletionStatusCookie, Value: deletion.StatusToken, Path: "/account/deletion", MaxAge: 86400, Expires: time.Now().Add(24 * time.Hour), HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteStrictMode})
		http.Redirect(w, r, "/account/deletion", http.StatusSeeOther)
		return
	}
	if deletion.Deleted {
		http.Redirect(w, r, "/login?account_deleted=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login?deletion_pending=1", http.StatusSeeOther)
}
