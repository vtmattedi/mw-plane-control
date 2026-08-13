package oidc

import "testing"

func TestDiscoveryUsesCanonicalIssuer(t *testing.T) {
	doc := Discovery(
		"https://identity.mattediworks.com/",
	)

	if doc.Issuer !=
		"https://identity.mattediworks.com" {
		t.Fatalf("issuer = %q", doc.Issuer)
	}
	if doc.JWKSURI !=
		"https://identity.mattediworks.com/.well-known/jwks.json" {
		t.Fatalf("jwks_uri = %q", doc.JWKSURI)
	}
	if len(doc.ResponseTypesSupported) != 1 ||
		doc.ResponseTypesSupported[0] != "code" {
		t.Fatalf(
			"response types = %#v",
			doc.ResponseTypesSupported,
		)
	}
}
