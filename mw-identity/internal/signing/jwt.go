package signing

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KID       string `json:"kid"`
}

func (m *Manager) SignJWT(
	ctx context.Context,
	claims map[string]any,
) (string, error) {
	signer, err := m.ActiveSigner(ctx)
	if err != nil {
		return "", err
	}

	headerJSON, err := json.Marshal(jwtHeader{
		Algorithm: signer.Algorithm,
		Type:      "JWT",
		KID:       signer.KID,
	})
	if err != nil {
		return "", fmt.Errorf("marshal JWT header: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal JWT claims: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := header + "." + payload

	digest := sha256.Sum256([]byte(signingInput))
	signature, err := signer.PrivateKey.Sign(
		rand.Reader,
		digest[:],
		crypto.SHA256,
	)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}

	return signingInput + "." +
		base64.RawURLEncoding.EncodeToString(signature), nil
}
