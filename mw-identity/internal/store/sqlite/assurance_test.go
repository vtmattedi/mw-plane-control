package sqlite

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	"strings"
	"testing"
	"time"
)

func TestSignedAssuranceRoundTripAndNoMintTimeUpgrade(t *testing.T) {
	f := newRemovalFixture(t, "")
	ctx := context.Background()
	ring, err := signing.ParseKeyring("1:"+base64.RawURLEncoding.EncodeToString(make([]byte, 32)), 1)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signing.NewManager(f.store, ring)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = signer.EnsureActiveKey(ctx); err != nil {
		t.Fatal(err)
	}
	provider, err := oidc.NewService(f.store, signer, oidc.ServiceConfig{Issuer: "https://identity.example", AuthorizationCodeTTL: time.Minute, AccessTokenTTL: time.Minute, IDTokenTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	user, err := f.store.IdentityUserByID(ctx, "usr_remove")
	if err != nil {
		t.Fatal(err)
	}
	verifier := strings.Repeat("v", 43)
	req := oidc.AuthorizationRequest{ResponseType: "code", ClientID: "one-client", RedirectURI: "https://example.com/callback", Scope: "openid", State: "bound-state", Nonce: "bound-nonce", CodeChallenge: oidc.S256Challenge(verifier), CodeChallengeMethod: "S256"}
	for _, c := range []struct{ proof, method, acr string }{{auth.ProofPassword, "password", auth.ACRStandard}, {auth.ProofMFA, "password_totp", auth.ACRStrong}, {auth.ProofPasskey, "webauthn", auth.ACRWebAuthn}} {
		req.ACRValues = c.acr
		validated, e := provider.ValidateAuthorization(ctx, req)
		if e != nil {
			t.Fatal(e)
		}
		authenticated := time.Now().Add(-30 * time.Second).UTC().Truncate(time.Second)
		code, err := provider.IssueAuthorizationCode(ctx, validated, user, authenticated, c.method, c.proof)
		if err != nil {
			t.Fatal(err)
		}
		result, e := provider.ExchangeCode(ctx, oidc.TokenRequest{GrantType: "authorization_code", ClientID: req.ClientID, Code: code, RedirectURI: req.RedirectURI, CodeVerifier: verifier})
		if e != nil {
			t.Fatal(e)
		}
		parts := strings.Split(result.IDToken, ".")
		signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
		key, err := signer.ActiveSigner(ctx)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if err = rsa.VerifyPKCS1v15(&key.PrivateKey.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
			t.Fatal(err)
		}
		raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var claims struct {
			ACR      string   `json:"acr"`
			AMR      []string `json:"amr"`
			AuthTime int64    `json:"auth_time"`
			Nonce    string   `json:"nonce"`
		}
		if err = json.Unmarshal(raw, &claims); err != nil {
			t.Fatal(err)
		}
		if claims.ACR != c.acr || claims.AuthTime != authenticated.Unix() || claims.Nonce != req.Nonce {
			t.Fatalf("incorrect signed evidence: %+v", claims)
		}
		if c.proof == auth.ProofMFA && strings.Join(claims.AMR, ",") != "pwd,otp" {
			t.Fatal("MFA factor lost")
		}
		if _, e = provider.ExchangeCode(ctx, oidc.TokenRequest{GrantType: "authorization_code", ClientID: req.ClientID, Code: code, RedirectURI: req.RedirectURI, CodeVerifier: verifier}); e == nil {
			t.Fatal("code replay")
		}
	}
	req.ACRValues = "urn:unknown"
	if _, e := provider.ValidateAuthorization(ctx, req); e == nil {
		t.Fatal("unknown ACR downgraded")
	}
	req.ACRValues = auth.ACRStrong
	v, e := provider.ValidateAuthorization(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	if _, err = provider.IssueAuthorizationCode(ctx, v, user, time.Now(), "password", auth.ProofPassword); err == nil {
		t.Fatal("password satisfied Strong")
	}
	if _, err = provider.IssueAuthorizationCode(ctx, v, user, time.Now(), "totp", ""); err == nil {
		t.Fatal("legacy TOTP proof promoted")
	}
	if _, err = provider.IssueAuthorizationCode(ctx, v, user, time.Now(), "webauthn", auth.ProofMFA); err == nil {
		t.Fatal("proof/method mismatch admitted as passkey provenance")
	}
	v.MaxAgeSet = true
	v.MaxAgeSeconds = 1
	if _, err = provider.IssueAuthorizationCode(ctx, v, user, time.Now().Add(-time.Minute), "webauthn", auth.ProofPasskey); err == nil {
		t.Fatal("stale proof accepted")
	}
}
