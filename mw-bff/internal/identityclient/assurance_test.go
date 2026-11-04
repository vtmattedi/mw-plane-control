package identityclient

import (
	"testing"
	"time"
)

func TestStepUpRejectsWrongSubjectStaleAndWeakerProof(t *testing.T) {
	now := time.Now().UTC()
	tx := Transaction{ExpectedSubject: "original", RequiredACR: "urn:mw:webauthn"}
	good := validatedClaims{Subject: "original", ACR: "urn:mw:webauthn", AuthMethod: "webauthn", AuthTime: now.Unix()}
	if err := validateStepUp(good, tx, now, time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*validatedClaims){func(c *validatedClaims) { c.Subject = "other" }, func(c *validatedClaims) { c.ACR = "urn:mw:auth:strong"; c.AuthMethod = "pwd" }, func(c *validatedClaims) { c.AuthTime = now.Add(-2 * time.Minute).Unix() }, func(c *validatedClaims) { c.ACR = "" }} {
		bad := good
		change(&bad)
		if err := validateStepUp(bad, tx, now, time.Minute); err == nil {
			t.Fatal("invalid step-up accepted")
		}
	}
}
