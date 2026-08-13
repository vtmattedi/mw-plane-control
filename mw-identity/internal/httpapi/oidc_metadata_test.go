package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoveryEndpoint(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/.well-known/openid-configuration",
		nil,
	)
	rec := httptest.NewRecorder()

	discoveryHandler(
		"https://identity.mattediworks.com",
	).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(
		rec.Body.String(),
		`"issuer":"https://identity.mattediworks.com"`,
	) {
		t.Fatalf(
			"unexpected body: %s",
			rec.Body.String(),
		)
	}
}
