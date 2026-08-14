package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
)

const strongWebAuthnACR = "urn:mw:webauthn"

func recentWebAuthnSession(session auth.Session, maxAge time.Duration, now time.Time) bool {
	if strings.TrimSpace(session.AuthMethod) != "webauthn" || session.AuthTime.IsZero() {
		return false
	}
	age := now.UTC().Sub(session.AuthTime.UTC())
	return age >= 0 && age <= maxAge
}

func recentWebAuthnAccess(access oidc.AccessContext, maxAge time.Duration, now time.Time) bool {
	return strings.TrimSpace(access.Token.AuthMethod) == "webauthn" &&
		!access.Token.AuthTime.IsZero() &&
		now.UTC().Sub(access.Token.AuthTime.UTC()) >= 0 &&
		now.UTC().Sub(access.Token.AuthTime.UTC()) <= maxAge
}

func strongSessionMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case "webauthn", "totp":
		return true
	default:
		return false
	}
}

func writeStepUpRequired(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, map[string]any{
		"error": map[string]any{
			"code": "step_up_required",
			"acr":  strongWebAuthnACR,
		},
	})
}
