package delegation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/secure"
)

var ErrNotFound = errors.New("delegated token not found")

const (
	tokenExchangeGrantType = "urn:ietf:params:oauth:grant-type:token-exchange"
	accessTokenType        = "urn:ietf:params:oauth:token-type:access_token"
)

type Config struct {
	IdentityBaseURL string
	ClientID        string
	ClientSecret    string
	HTTPTimeout     time.Duration
	CacheSkew       time.Duration
}

type Client struct {
	config     Config
	store      Store
	keyring    secure.Keyring
	httpClient *http.Client
	now        func() time.Time
}

type exchangeResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int64  `json:"expires_in"`
	Scope           string `json:"scope"`
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
		return nil, fmt.Errorf("delegation store is required")
	}
	config.IdentityBaseURL = strings.TrimRight(
		strings.TrimSpace(config.IdentityBaseURL),
		"/",
	)
	if config.IdentityBaseURL == "" ||
		strings.TrimSpace(config.ClientID) == "" ||
		config.ClientSecret == "" {
		return nil, fmt.Errorf("delegation Identity configuration is incomplete")
	}
	if config.HTTPTimeout <= 0 {
		return nil, fmt.Errorf("delegation HTTP timeout must be positive")
	}
	if config.CacheSkew < 0 {
		return nil, fmt.Errorf("delegation cache skew cannot be negative")
	}
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

func (c *Client) Token(
	ctx context.Context,
	adminSessionID string,
	subjectToken string,
	audience string,
	scopes []string,
) (Token, error) {
	adminSessionID = strings.TrimSpace(adminSessionID)
	subjectToken = strings.TrimSpace(subjectToken)
	audience = strings.TrimSpace(audience)
	scope := canonicalScope(scopes)
	if adminSessionID == "" || subjectToken == "" ||
		audience == "" || scope == "" {
		return Token{}, fmt.Errorf("delegation request is incomplete")
	}

	now := c.now().UTC()
	cacheNow := now.Add(c.config.CacheSkew)
	cached, err := c.store.DelegatedToken(
		ctx,
		adminSessionID,
		audience,
		scope,
		cacheNow,
	)
	if err == nil {
		raw, decryptErr := c.decrypt(cached)
		if decryptErr != nil {
			return Token{}, decryptErr
		}
		return Token{
			AccessToken: raw,
			ExpiresAt:   cached.ExpiresAt,
			Cached:      true,
		}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Token{}, err
	}

	exchanged, err := c.exchange(
		ctx,
		subjectToken,
		audience,
		scope,
	)
	if err != nil {
		return Token{}, err
	}
	if exchanged.ExpiresIn <= 0 {
		return Token{}, fmt.Errorf("Identity returned invalid delegated token lifetime")
	}

	expiresAt := now.Add(time.Duration(exchanged.ExpiresIn) * time.Second)
	record, err := c.encrypt(
		adminSessionID,
		audience,
		scope,
		exchanged.AccessToken,
		expiresAt,
		now,
	)
	if err != nil {
		return Token{}, err
	}
	if err := c.store.UpsertDelegatedToken(ctx, record); err != nil {
		return Token{}, err
	}
	_ = c.store.DeleteExpiredDelegatedTokens(ctx, now)

	return Token{
		AccessToken: exchanged.AccessToken,
		ExpiresAt:   expiresAt,
		Cached:      false,
	}, nil
}

func (c *Client) exchange(
	ctx context.Context,
	subjectToken string,
	audience string,
	scope string,
) (exchangeResponse, error) {
	form := url.Values{}
	form.Set("grant_type", tokenExchangeGrantType)
	form.Set("subject_token", subjectToken)
	form.Set("subject_token_type", accessTokenType)
	form.Set("requested_token_type", accessTokenType)
	form.Set("audience", audience)
	form.Set("scope", scope)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.config.IdentityBaseURL+"/oauth/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return exchangeResponse{}, fmt.Errorf("build token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.config.ClientID, c.config.ClientSecret)

	response, err := c.httpClient.Do(req)
	if err != nil {
		return exchangeResponse{}, fmt.Errorf("Identity token exchange failed: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return exchangeResponse{}, fmt.Errorf("read token exchange response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure oauthError
		_ = json.Unmarshal(body, &failure)
		return exchangeResponse{}, fmt.Errorf(
			"Identity token exchange returned %d (%s)",
			response.StatusCode,
			failure.Error,
		)
	}

	var token exchangeResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&token); err != nil {
		return exchangeResponse{}, fmt.Errorf("decode token exchange response: %w", err)
	}
	if token.AccessToken == "" ||
		token.TokenType != "Bearer" ||
		token.IssuedTokenType != accessTokenType {
		return exchangeResponse{}, fmt.Errorf("Identity returned invalid token exchange response")
	}
	return token, nil
}

func (c *Client) encrypt(
	sessionID string,
	audience string,
	scope string,
	rawToken string,
	expiresAt time.Time,
	now time.Time,
) (CacheRecord, error) {
	id, err := randomID("del_")
	if err != nil {
		return CacheRecord{}, err
	}
	plaintext := []byte(rawToken)
	defer zero(plaintext)
	aad := "mw-bff-delegated-token:" + sessionID + ":" + audience + ":" + scope
	ciphertext, nonce, version, err := c.keyring.Encrypt(aad, plaintext)
	if err != nil {
		return CacheRecord{}, err
	}
	return CacheRecord{
		ID:                   id,
		AdminSessionID:       sessionID,
		Audience:             audience,
		Scope:                scope,
		TokenCiphertext:      ciphertext,
		TokenNonce:           nonce,
		EncryptionKeyVersion: version,
		ExpiresAt:            expiresAt,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

func (c *Client) decrypt(record CacheRecord) (string, error) {
	aad := "mw-bff-delegated-token:" + record.AdminSessionID + ":" + record.Audience + ":" + record.Scope
	plaintext, err := c.keyring.Decrypt(
		record.EncryptionKeyVersion,
		aad,
		record.TokenNonce,
		record.TokenCiphertext,
	)
	if err != nil {
		return "", err
	}
	defer zero(plaintext)
	return string(plaintext), nil
}

func canonicalScope(scopes []string) string {
	seen := make(map[string]struct{})
	values := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		values = append(values, scope)
	}
	sort.Strings(values)
	return strings.Join(values, " ")
}

func randomID(prefix string) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate delegation id: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
