package oidc

import "testing"

func TestS256RFC7636Vector(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	if got := S256Challenge(verifier); got != want {
		t.Fatalf("challenge = %q; want %q", got, want)
	}
	if !ValidCodeVerifier(verifier) {
		t.Fatal("valid verifier rejected")
	}
	if !ValidS256Challenge(want) {
		t.Fatal("valid challenge rejected")
	}
}
