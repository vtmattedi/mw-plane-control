package auth

import "testing"

func TestAssuranceSatisfactionMatrix(t *testing.T) {
	for _, c := range []struct {
		proof, acr string
		want       bool
	}{{ProofPassword, ACRStandard, true}, {ProofPassword, ACRStrong, false}, {ProofMFA, ACRStrong, true}, {ProofMFA, ACRWebAuthn, false}, {ProofPasskey, ACRWebAuthn, true}, {ProofPasskey, ACRStrong, true}, {ProofPasskey, ACRStandard, true}, {"totp", ACRStrong, false}, {"recovery", ACRStandard, false}, {"", ACRStandard, false}, {ProofPasskey, "unknown", false}} {
		if got := Satisfies(c.proof, c.acr); got != c.want {
			t.Fatalf("%s satisfies %s = %v", c.proof, c.acr, got)
		}
	}
}
