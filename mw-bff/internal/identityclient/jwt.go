package identityclient

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type idTokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KID       string `json:"kid"`
}

type idTokenClaims struct {
	Issuer      string          `json:"iss"`
	Subject     string          `json:"sub"`
	Audience    json.RawMessage `json:"aud"`
	Expires     int64           `json:"exp"`
	IssuedAt    int64           `json:"iat"`
	AuthTime    int64           `json:"auth_time"`
	AMR         []string        `json:"amr"`
	Nonce       string          `json:"nonce"`
	Email       string          `json:"email"`
	Name        string          `json:"name"`
	AccountType string          `json:"mw_account_type"`
}

type publicJWKS struct {
	Keys []publicJWK `json:"keys"`
}

type publicJWK struct {
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	KID       string `json:"kid"`
	N         string `json:"n"`
	E         string `json:"e"`
}

type validatedClaims struct {
	Issuer      string
	Subject     string
	AuthTime    int64
	AuthMethod  string
	Nonce       string
	Email       string
	Name        string
	AccountType string
}

func (c *Client) validateIDToken(
	ctx context.Context,
	token string,
	now time.Time,
) (validatedClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return validatedClaims{},
			fmt.Errorf(
				"%w: invalid ID token format",
				ErrIdentityProtocol,
			)
	}

	headerRaw, err := base64.RawURLEncoding.DecodeString(
		parts[0],
	)
	if err != nil {
		return validatedClaims{},
			fmt.Errorf(
				"%w: invalid ID token header",
				ErrIdentityProtocol,
			)
	}

	var header idTokenHeader
	if err := json.Unmarshal(
		headerRaw,
		&header,
	); err != nil {
		return validatedClaims{},
			fmt.Errorf(
				"%w: invalid ID token header JSON",
				ErrIdentityProtocol,
			)
	}

	if header.Algorithm != "RS256" ||
		header.KID == "" {
		return validatedClaims{},
			fmt.Errorf(
				"%w: unsupported ID token algorithm/key",
				ErrIdentityProtocol,
			)
	}

	key, err := c.fetchJWK(
		ctx,
		header.KID,
	)
	if err != nil {
		return validatedClaims{}, err
	}

	publicKey, err := rsaPublicKey(key)
	if err != nil {
		return validatedClaims{}, err
	}

	signature, err := base64.RawURLEncoding.DecodeString(
		parts[2],
	)
	if err != nil {
		return validatedClaims{},
			fmt.Errorf(
				"%w: invalid ID token signature encoding",
				ErrIdentityProtocol,
			)
	}

	digest := sha256.Sum256(
		[]byte(parts[0] + "." + parts[1]),
	)

	if err := rsa.VerifyPKCS1v15(
		publicKey,
		crypto.SHA256,
		digest[:],
		signature,
	); err != nil {
		return validatedClaims{},
			fmt.Errorf(
				"%w: ID token signature verification failed",
				ErrIdentityProtocol,
			)
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(
		parts[1],
	)
	if err != nil {
		return validatedClaims{},
			fmt.Errorf(
				"%w: invalid ID token payload",
				ErrIdentityProtocol,
			)
	}

	var claims idTokenClaims
	if err := json.Unmarshal(
		payloadRaw,
		&claims,
	); err != nil {
		return validatedClaims{},
			fmt.Errorf(
				"%w: invalid ID token claims",
				ErrIdentityProtocol,
			)
	}

	if claims.Issuer != c.config.Issuer {
		return validatedClaims{},
			fmt.Errorf(
				"%w: issuer mismatch",
				ErrIdentityProtocol,
			)
	}
	if !audienceContains(
		claims.Audience,
		c.config.ClientID,
	) {
		return validatedClaims{},
			fmt.Errorf(
				"%w: audience mismatch",
				ErrIdentityProtocol,
			)
	}

	nowUnix := now.Unix()
	skew := int64(c.config.ClockSkew.Seconds())

	if claims.Expires <= 0 ||
		nowUnix > claims.Expires+skew {
		return validatedClaims{},
			fmt.Errorf(
				"%w: ID token expired",
				ErrIdentityProtocol,
			)
	}
	if claims.IssuedAt <= 0 ||
		claims.IssuedAt > nowUnix+skew {
		return validatedClaims{},
			fmt.Errorf(
				"%w: invalid iat",
				ErrIdentityProtocol,
			)
	}
	if claims.AuthTime <= 0 ||
		claims.AuthTime > nowUnix+skew {
		return validatedClaims{},
			fmt.Errorf(
				"%w: invalid auth_time",
				ErrIdentityProtocol,
			)
	}
	if claims.Subject == "" ||
		claims.Nonce == "" {
		return validatedClaims{},
			fmt.Errorf(
				"%w: required ID token claim missing",
				ErrIdentityProtocol,
			)
	}

	authMethod := "unknown"
	if len(claims.AMR) > 0 {
		authMethod = strings.TrimSpace(claims.AMR[0])
	}
	return validatedClaims{
		Issuer:      claims.Issuer,
		Subject:     claims.Subject,
		AuthTime:    claims.AuthTime,
		AuthMethod:  authMethod,
		Nonce:       claims.Nonce,
		Email:       claims.Email,
		Name:        claims.Name,
		AccountType: claims.AccountType,
	}, nil
}

func (c *Client) fetchJWK(
	ctx context.Context,
	kid string,
) (publicJWK, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.config.InternalBaseURL+
			"/.well-known/jwks.json",
		nil,
	)
	if err != nil {
		return publicJWK{}, fmt.Errorf(
			"build JWKS request: %w",
			err,
		)
	}

	response, err := c.httpClient.Do(req)
	if err != nil {
		return publicJWK{}, fmt.Errorf(
			"fetch Identity JWKS: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return publicJWK{}, fmt.Errorf(
			"%w: JWKS returned HTTP %d",
			ErrIdentityProtocol,
			response.StatusCode,
		)
	}

	body, err := io.ReadAll(
		io.LimitReader(
			response.Body,
			1024*1024,
		),
	)
	if err != nil {
		return publicJWK{}, fmt.Errorf(
			"read Identity JWKS: %w",
			err,
		)
	}

	var set publicJWKS
	if err := json.Unmarshal(
		body,
		&set,
	); err != nil {
		return publicJWK{}, fmt.Errorf(
			"%w: invalid JWKS",
			ErrIdentityProtocol,
		)
	}

	for _, key := range set.Keys {
		if key.KID == kid &&
			key.KeyType == "RSA" &&
			key.Use == "sig" &&
			key.Algorithm == "RS256" {
			return key, nil
		}
	}

	return publicJWK{},
		fmt.Errorf(
			"%w: signing kid not found",
			ErrIdentityProtocol,
		)
}

func rsaPublicKey(
	key publicJWK,
) (*rsa.PublicKey, error) {
	modulus, err := base64.RawURLEncoding.DecodeString(
		key.N,
	)
	if err != nil || len(modulus) == 0 {
		return nil, fmt.Errorf(
			"%w: invalid RSA modulus",
			ErrIdentityProtocol,
		)
	}

	exponentBytes, err :=
		base64.RawURLEncoding.DecodeString(
			key.E,
		)
	if err != nil ||
		len(exponentBytes) == 0 ||
		len(exponentBytes) > 4 {
		return nil, fmt.Errorf(
			"%w: invalid RSA exponent",
			ErrIdentityProtocol,
		)
	}

	padded := make([]byte, 4)
	copy(
		padded[4-len(exponentBytes):],
		exponentBytes,
	)
	exponent := binary.BigEndian.Uint32(padded)
	if exponent < 3 {
		return nil, fmt.Errorf(
			"%w: invalid RSA exponent",
			ErrIdentityProtocol,
		)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulus),
		E: int(exponent),
	}, nil
}

func audienceContains(
	raw json.RawMessage,
	expected string,
) bool {
	var single string
	if err := json.Unmarshal(
		raw,
		&single,
	); err == nil {
		return single == expected
	}

	var multiple []string
	if err := json.Unmarshal(
		raw,
		&multiple,
	); err != nil {
		return false
	}

	for _, value := range multiple {
		if value == expected {
			return true
		}
	}

	return false
}
