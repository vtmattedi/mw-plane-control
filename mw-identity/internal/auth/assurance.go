package auth

const (
	ACRStandard   = "urn:mw:auth:standard"
	ACRStrong     = "urn:mw:auth:strong"
	ACRWebAuthn   = "urn:mw:webauthn"
	ProofPassword = "pwd_v1"
	ProofPasskey  = "webauthn_uv_v1"
	ProofMFA      = "pwd_totp_v1"
)

func Assurance(proof string) (string, []string) {
	switch proof {
	case ProofPassword:
		return ACRStandard, []string{"pwd"}
	case ProofPasskey:
		return ACRWebAuthn, []string{"webauthn"}
	case ProofMFA:
		return ACRStrong, []string{"pwd", "otp"}
	default:
		return "", nil
	}
}
func Satisfies(proof, requested string) bool {
	achieved, _ := Assurance(proof)
	switch requested {
	case ACRStandard:
		return achieved != ""
	case ACRStrong:
		return achieved == ACRStrong || achieved == ACRWebAuthn
	case ACRWebAuthn:
		return achieved == ACRWebAuthn
	default:
		return false
	}
}

func MethodForProof(proof string) string {
	switch proof {
	case ProofPassword:
		return "password"
	case ProofMFA:
		return "password_totp"
	case ProofPasskey:
		return "webauthn"
	}
	return ""
}
