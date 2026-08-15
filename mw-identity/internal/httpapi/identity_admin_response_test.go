package httpapi

import (
	"testing"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
)

func TestApplicationCreationResponseOmitsEmptyPublicClientSecret(t *testing.T) {
	public := applicationCreationResponse(identityadmin.CreateApplicationResult{
		Application: applications.Application{
			ClientID:        "mw-woi",
			ApplicationType: "public",
		},
	})
	if _, exists := public["client_secret"]; exists {
		t.Fatal("public application response contains an empty client secret")
	}
	app := public["application"].(applications.Application)
	if app.ClientID != "mw-woi" {
		t.Fatalf("client ID = %q", app.ClientID)
	}

	confidential := applicationCreationResponse(identityadmin.CreateApplicationResult{
		Application:  applications.Application{ApplicationType: "confidential"},
		ClientSecret: "mw_client_secret",
	})
	if confidential["client_secret"] != "mw_client_secret" {
		t.Fatal("confidential application response omitted its issued secret")
	}
}
