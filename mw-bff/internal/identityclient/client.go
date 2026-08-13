package identityclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/secure"
)

type Config struct {
	Issuer          string
	InternalBaseURL string
	ClientID        string
	ClientSecret    string
	CallbackURL     string
	Scopes          []string
	TransactionTTL  time.Duration
	ClockSkew       time.Duration
	HTTPTimeout     time.Duration
}

type Client struct {
	config     Config
	store      Store
	keyring    secure.Keyring
	httpClient *http.Client
	now        func() time.Time
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
	IDToken     string `json:"id_token"`
}

type oauthError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func New(
	config Config,
	store Store,
	keyring secure.Keyring,
) (*Client, error) {
	if store == nil {
		return nil, fmt.Errorf(
			"Identity transaction store is required",
		)
	}
	if strings.TrimSpace(config.Issuer) == "" ||
		strings.TrimSpace(config.InternalBaseURL) == "" ||
		strings.TrimSpace(config.ClientID) == "" ||
		config.ClientSecret == "" ||
		strings.TrimSpace(config.CallbackURL) == "" {
		return nil, fmt.Errorf(
			"Identity OIDC client configuration is incomplete",
		)
	}
	if config.TransactionTTL <= 0 ||
		config.HTTPTimeout <= 0 {
		return nil, fmt.Errorf(
			"Identity OIDC durations must be positive",
		)
	}
	if len(config.Scopes) == 0 {
		return nil, fmt.Errorf(
			"Identity OIDC scopes are required",
		)
	}

	config.Issuer = strings.TrimRight(
		config.Issuer,
		"/",
	)
	config.InternalBaseURL = strings.TrimRight(
		config.InternalBaseURL,
		"/",
	)

	return &Client{
		config:  config,
		store:   store,
		keyring: keyring,
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
			CheckRedirect: func(
				req *http.Request,
				via []*http.Request,
			) error {
				return http.ErrUseLastResponse
			},
		},
		now: time.Now,
	}, nil
}

func (c *Client) Start(
	ctx context.Context,
	returnTo string,
) (string, error) {
	transactionID, err := randomOpaque("oidctx_", 18)
	if err != nil {
		return "", err
	}

	state, err := randomOpaque("", 32)
	if err != nil {
		return "", err
	}
	nonce, err := randomOpaque("", 32)
	if err != nil {
		return "", err
	}
	verifier, err := randomOpaque("", 64)
	if err != nil {
		return "", err
	}

	challengeSum := sha256.Sum256(
		[]byte(verifier),
	)
	challenge := base64.RawURLEncoding.EncodeToString(
		challengeSum[:],
	)

	verifierBytes := []byte(verifier)
	defer zero(verifierBytes)

	ciphertext, cipherNonce, keyVersion, err :=
		c.keyring.Encrypt(
			"mw-bff-oidc:"+transactionID,
			verifierBytes,
		)
	if err != nil {
		return "", err
	}

	now := c.now().UTC()
	err = c.store.CreateOIDCTransaction(
		ctx,
		Transaction{
			ID:                     transactionID,
			StateHash:              hashOpaque(state),
			NonceHash:              hashOpaque(nonce),
			PKCEVerifierCiphertext: ciphertext,
			PKCEVerifierNonce:      cipherNonce,
			EncryptionKeyVersion:   keyVersion,
			ReturnTo:               returnTo,
			ExpiresAt:              now.Add(c.config.TransactionTTL),
			CreatedAt:              now,
		},
	)
	if err != nil {
		return "", err
	}

	_ = c.store.DeleteExpiredOIDCTransactions(
		ctx,
		now,
	)

	target, err := url.Parse(
		c.config.Issuer + "/oauth/authorize",
	)
	if err != nil {
		return "", fmt.Errorf(
			"build Identity authorization URL: %w",
			err,
		)
	}

	query := target.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.config.ClientID)
	query.Set("redirect_uri", c.config.CallbackURL)
	query.Set(
		"scope",
		strings.Join(c.config.Scopes, " "),
	)
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	target.RawQuery = query.Encode()

	return target.String(), nil
}

func (c *Client) Complete(
	ctx context.Context,
	code string,
	state string,
) (Identity, string, error) {
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" || state == "" {
		return Identity{}, "", ErrIdentityProtocol
	}

	now := c.now().UTC()
	transaction, err :=
		c.store.ClaimOIDCTransaction(
			ctx,
			hashOpaque(state),
			now,
		)
	if err != nil {
		if errors.Is(
			err,
			ErrTransactionNotFound,
		) {
			return Identity{}, "", ErrTransactionNotFound
		}
		return Identity{}, "", err
	}

	verifierBytes, err := c.keyring.Decrypt(
		transaction.EncryptionKeyVersion,
		"mw-bff-oidc:"+transaction.ID,
		transaction.PKCEVerifierNonce,
		transaction.PKCEVerifierCiphertext,
	)
	if err != nil {
		return Identity{}, "", fmt.Errorf(
			"decrypt PKCE verifier: %w",
			err,
		)
	}
	defer zero(verifierBytes)

	token, err := c.exchangeCode(
		ctx,
		code,
		string(verifierBytes),
	)
	if err != nil {
		return Identity{}, "", err
	}

	if token.TokenType != "Bearer" ||
		token.AccessToken == "" ||
		token.IDToken == "" {
		return Identity{}, "", ErrIdentityProtocol
	}

	claims, err := c.validateIDToken(
		ctx,
		token.IDToken,
		now,
	)
	if err != nil {
		return Identity{}, "", err
	}

	if !constantTimeEqual(
		hashOpaque(claims.Nonce),
		transaction.NonceHash,
	) {
		return Identity{}, "", fmt.Errorf(
			"%w: nonce mismatch",
			ErrIdentityProtocol,
		)
	}

	if claims.Subject == "" {
		return Identity{}, "", fmt.Errorf(
			"%w: subject missing",
			ErrIdentityProtocol,
		)
	}

	authTime := time.Unix(
		claims.AuthTime,
		0,
	).UTC()

	return Identity{
		Issuer:      claims.Issuer,
		Subject:     claims.Subject,
		Email:       claims.Email,
		DisplayName: claims.Name,
		AuthTime:    authTime,
	}, transaction.ReturnTo, nil
}

func (c *Client) exchangeCode(
	ctx context.Context,
	code string,
	verifier string,
) (tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", c.config.CallbackURL)
	form.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.config.InternalBaseURL+"/oauth/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return tokenResponse{}, fmt.Errorf(
			"build token request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)
	req.SetBasicAuth(
		c.config.ClientID,
		c.config.ClientSecret,
	)

	response, err := c.httpClient.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf(
			"Identity token request failed: %w",
			err,
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(
		io.LimitReader(
			response.Body,
			1024*1024,
		),
	)
	if err != nil {
		return tokenResponse{}, fmt.Errorf(
			"read Identity token response: %w",
			err,
		)
	}

	if response.StatusCode < 200 ||
		response.StatusCode >= 300 {
		var oauthFailure oauthError
		_ = json.Unmarshal(body, &oauthFailure)

		return tokenResponse{}, fmt.Errorf(
			"%w: token endpoint returned %d (%s)",
			ErrIdentityProtocol,
			response.StatusCode,
			oauthFailure.Error,
		)
	}

	var token tokenResponse
	if err := json.NewDecoder(
		bytes.NewReader(body),
	).Decode(&token); err != nil {
		return tokenResponse{}, fmt.Errorf(
			"decode Identity token response: %w",
			err,
		)
	}

	return token, nil
}

func randomOpaque(
	prefix string,
	byteCount int,
) (string, error) {
	raw := make([]byte, byteCount)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf(
			"generate OIDC random value: %w",
			err,
		)
	}

	return prefix +
		base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashOpaque(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(left),
		[]byte(right),
	) == 1
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
