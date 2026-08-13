package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OIDCClient struct {
	issuer       string
	browserBase  string
	internalBase string
	hostHeader   string
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
	now          func() time.Time
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	IDToken     string `json:"id_token"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KID       string `json:"kid"`
}

type jwtClaims struct {
	Issuer      string          `json:"iss"`
	Subject     string          `json:"sub"`
	Audience    json.RawMessage `json:"aud"`
	Expires     int64           `json:"exp"`
	IssuedAt    int64           `json:"iat"`
	Nonce       string          `json:"nonce"`
	Email       string          `json:"email"`
	DisplayName string          `json:"name"`
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	KID       string `json:"kid"`
	N         string `json:"n"`
	E         string `json:"e"`
}

func NewOIDCClient(cfg Config) *OIDCClient {
	return &OIDCClient{
		issuer:       cfg.IdentityIssuer,
		browserBase:  cfg.IdentityBrowserBase,
		internalBase: cfg.IdentityInternalBase,
		hostHeader:   cfg.IdentityHost,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURI:  cfg.PublicBaseURL + "/oidc/callback",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now: time.Now,
	}
}

func (c *OIDCClient) AuthorizationURL(state, nonce, verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	target, _ := url.Parse(c.browserBase + "/oauth/authorize")
	q := target.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("scope", "openid profile email")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	target.RawQuery = q.Encode()
	return target.String()
}

func (c *OIDCClient) ExchangeAndValidate(ctx context.Context, code, verifier, expectedNonceHash string) (IdentityClaims, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", c.redirectURI)
	form.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.internalBase+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return IdentityClaims{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.hostHeader != "" {
		req.Host = c.hostHeader
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)

	response, err := c.httpClient.Do(req)
	if err != nil {
		return IdentityClaims{}, fmt.Errorf("token exchange: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return IdentityClaims{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return IdentityClaims{}, fmt.Errorf("token endpoint returned %d", response.StatusCode)
	}
	var token tokenResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&token); err != nil {
		return IdentityClaims{}, err
	}
	if token.TokenType != "Bearer" || token.IDToken == "" {
		return IdentityClaims{}, fmt.Errorf("invalid token response")
	}
	claims, err := c.validateIDToken(ctx, token.IDToken, expectedNonceHash)
	if err != nil {
		return IdentityClaims{}, err
	}
	return IdentityClaims{
		Issuer:      claims.Issuer,
		Subject:     claims.Subject,
		Email:       claims.Email,
		DisplayName: claims.DisplayName,
	}, nil
}

func (c *OIDCClient) validateIDToken(ctx context.Context, raw, expectedNonceHash string) (jwtClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return jwtClaims{}, fmt.Errorf("invalid JWT shape")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtClaims{}, err
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtClaims{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtClaims{}, err
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return jwtClaims{}, err
	}
	if header.Algorithm != "RS256" || header.KID == "" {
		return jwtClaims{}, fmt.Errorf("unexpected JWT algorithm or missing kid")
	}
	key, err := c.signingKey(ctx, header.KID)
	if err != nil {
		return jwtClaims{}, err
	}
	signed := []byte(parts[0] + "." + parts[1])
	digest := sha256.Sum256(signed)
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return jwtClaims{}, fmt.Errorf("verify ID token signature: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return jwtClaims{}, err
	}
	now := c.now().UTC().Unix()
	if claims.Issuer != c.issuer || claims.Subject == "" {
		return jwtClaims{}, fmt.Errorf("ID token issuer/subject invalid")
	}
	if claims.Expires <= now-60 || claims.IssuedAt > now+60 {
		return jwtClaims{}, fmt.Errorf("ID token time claims invalid")
	}
	if !audienceContains(claims.Audience, c.clientID) {
		return jwtClaims{}, fmt.Errorf("ID token audience mismatch")
	}
	actualNonceHash := hashOpaque(claims.Nonce)
	if len(actualNonceHash) != len(expectedNonceHash) ||
		subtle.ConstantTimeCompare([]byte(actualNonceHash), []byte(expectedNonceHash)) != 1 {
		return jwtClaims{}, fmt.Errorf("ID token nonce mismatch")
	}
	return claims, nil
}

func (c *OIDCClient) signingKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.internalBase+"/.well-known/jwks.json", nil)
	if err != nil {
		return nil, err
	}
	if c.hostHeader != "" {
		req.Host = c.hostHeader
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS returned %d", response.StatusCode)
	}
	var document jwksDocument
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&document); err != nil {
		return nil, err
	}
	for _, value := range document.Keys {
		if value.KID != kid || value.KeyType != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(value.N)
		if err != nil {
			return nil, err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(value.E)
		if err != nil || len(eBytes) == 0 || len(eBytes) > 4 {
			return nil, fmt.Errorf("invalid RSA exponent")
		}
		var padded [4]byte
		copy(padded[4-len(eBytes):], eBytes)
		exponent := binary.BigEndian.Uint32(padded[:])
		if exponent == 0 {
			return nil, fmt.Errorf("invalid RSA exponent")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(exponent)}, nil
	}
	return nil, fmt.Errorf("signing key %q not found", kid)
}

func audienceContains(raw json.RawMessage, expected string) bool {
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return one == expected
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return false
	}
	for _, value := range many {
		if value == expected {
			return true
		}
	}
	return false
}
