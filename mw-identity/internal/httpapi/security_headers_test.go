package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersAllowInlineTOTPQRCodeImages(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/account", nil))

	policy := recorder.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "img-src 'self' data:") {
		t.Fatalf("CSP does not allow the inline TOTP QR image: %q", policy)
	}
}
