package oidc

import "testing"

func TestIdentityOnlyScopesCannotBeDelegated(t *testing.T) {
	for _, scope := range []string{
		"openid",
		"profile",
		"email",
		"identity.admin",
	} {
		if !isIdentityOnlyScope(scope) {
			t.Fatalf("scope %q was not treated as Identity-only", scope)
		}
	}
	if isIdentityOnlyScope("oauth.projects.read") {
		t.Fatal("service permission was treated as Identity-only")
	}
}
