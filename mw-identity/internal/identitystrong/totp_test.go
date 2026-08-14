package identitystrong

import (
	"testing"
	"time"
)

func TestTOTPWindowAndReplay(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1700000000, 0).UTC()
	step := now.Unix() / 30
	code := totpAtStep(secret, step)

	accepted, ok := ValidateTOTP(secret, code, now, -1)
	if !ok || accepted != step {
		t.Fatalf("code not accepted: step=%d ok=%v", accepted, ok)
	}
	if _, ok := ValidateTOTP(secret, code, now, accepted); ok {
		t.Fatal("same TOTP timestep was replayable")
	}

	previous := totpAtStep(secret, step-1)
	if _, ok := ValidateTOTP(secret, previous, now, -1); !ok {
		t.Fatal("previous timestep inside +/-1 window rejected")
	}
}
