package httpapi

import (
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

const accountReauthenticationURL = "/login?auth=reauth&return_to=%2Faccount%23delete-account"

func (h *identityAuthHandlers) recentAccountAuthentication(session auth.Session) bool {
	maxAge := h.config.StrongAuthMaxAge
	if maxAge <= 0 {
		maxAge = 5 * time.Minute
	}
	age := time.Since(session.AuthTime)
	return auth.Satisfies(session.Proof, auth.ACRStandard) &&
		auth.MethodForProof(session.Proof) == session.AuthMethod &&
		!session.AuthTime.IsZero() && age >= 0 && age <= maxAge
}
