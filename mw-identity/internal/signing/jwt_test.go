package signing

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestSignJWTVerifiesWithPublicKey(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = 12
	}

	keyring, err := ParseKeyring(
		"1:"+base64.RawURLEncoding.EncodeToString(raw),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	store := &memorySigningStore{}
	manager, err := NewManager(store, keyring)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.EnsureActiveKey(context.Background()); err != nil {
		t.Fatal(err)
	}

	token, err := manager.SignJWT(
		context.Background(),
		map[string]any{
			"iss": "https://identity.mattediworks.com",
			"sub": "usr_test",
			"aud": "client_test",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT parts = %d; want 3", len(parts))
	}

	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}

	var header struct {
		Alg string `json:"alg"`
		KID string `json:"kid"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		t.Fatal(err)
	}
	if header.Alg != "RS256" || header.KID == "" {
		t.Fatalf("unexpected header: %#v", header)
	}

	signer, err := manager.ActiveSigner(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))

	if err := rsa.VerifyPKCS1v15(
		&signer.PrivateKey.PublicKey,
		crypto.SHA256,
		digest[:],
		signature,
	); err != nil {
		t.Fatalf("JWT signature did not verify: %v", err)
	}
}
