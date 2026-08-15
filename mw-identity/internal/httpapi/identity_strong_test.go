package httpapi

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strings"
	"testing"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
)

func TestTOTPSetupResponseIncludesScannablePNG(t *testing.T) {
	setup := identitystrong.TOTPSetup{
		Secret:     "JBSWY3DPEHPK3PXP",
		OTPAuthURI: "otpauth://totp/MW%20Identity:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=MW+Identity",
	}
	payload, err := newTOTPSetupResponse(setup)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Secret != setup.Secret || payload.OTPAuthURI != setup.OTPAuthURI {
		t.Fatal("TOTP provisioning fields changed while generating QR code")
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(payload.QRCodeDataURL, prefix) {
		t.Fatalf("QR code is not an inline PNG: %q", payload.QRCodeDataURL)
	}
	imageBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(payload.QRCodeDataURL, prefix))
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != totpQRCodeSize || config.Height != totpQRCodeSize {
		t.Fatalf("QR dimensions = %dx%d", config.Width, config.Height)
	}
}
