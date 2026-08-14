package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
)

type ServiceConfig struct {
	Issuer               string
	AuthorizationCodeTTL time.Duration
	AccessTokenTTL       time.Duration
	IDTokenTTL           time.Duration
	DelegatedTokenTTL    time.Duration
}

type Service struct {
	store   Store
	signing *signing.Manager
	config  ServiceConfig
	now     func() time.Time
}

func NewService(
	store Store,
	signingManager *signing.Manager,
	config ServiceConfig,
) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("OIDC store is required")
	}
	if signingManager == nil {
		return nil, fmt.Errorf("OIDC signing manager is required")
	}
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, fmt.Errorf("OIDC issuer is required")
	}
	if config.AuthorizationCodeTTL <= 0 ||
		config.AccessTokenTTL <= 0 ||
		config.IDTokenTTL <= 0 {
		return nil, fmt.Errorf("OIDC TTLs must be positive")
	}

	if config.DelegatedTokenTTL <= 0 {
		config.DelegatedTokenTTL = 5 * time.Minute
	}

	config.Issuer = strings.TrimRight(config.Issuer, "/")

	return &Service{
		store:   store,
		signing: signingManager,
		config:  config,
		now:     time.Now,
	}, nil
}

func (s *Service) ValidateAuthorization(
	ctx context.Context,
	request AuthorizationRequest,
) (ValidatedAuthorization, *ProtocolError) {
	clientID := strings.TrimSpace(request.ClientID)
	if clientID == "" {
		return ValidatedAuthorization{},
			invalidRequest("client_id is required", false)
	}

	app, err := s.store.ApplicationByClientID(ctx, clientID)
	if err != nil {
		return ValidatedAuthorization{},
			invalidRequest("unknown client_id", false)
	}
	if !app.Enabled || app.ArchivedAt != nil {
		return ValidatedAuthorization{},
			invalidRequest("client is disabled", false)
	}

	redirectURI := strings.TrimSpace(request.RedirectURI)
	if !contains(app.RedirectURIs, redirectURI) {
		return ValidatedAuthorization{},
			invalidRequest("redirect_uri is not registered", false)
	}

	if request.ResponseType != "code" {
		return ValidatedAuthorization{},
			invalidRequest("response_type must be code", true)
	}

	state := strings.TrimSpace(request.State)
	if state == "" || len(state) > 1024 {
		return ValidatedAuthorization{},
			invalidRequest("state is required", true)
	}

	if request.CodeChallengeMethod != "S256" {
		return ValidatedAuthorization{},
			invalidRequest(
				"code_challenge_method must be S256",
				true,
			)
	}
	if !ValidS256Challenge(request.CodeChallenge) {
		return ValidatedAuthorization{},
			invalidRequest("invalid S256 code_challenge", true)
	}

	nonce := strings.TrimSpace(request.Nonce)
	if len(nonce) > 1024 {
		return ValidatedAuthorization{},
			invalidRequest("nonce is too long", true)
	}

	scopes := parseScope(request.Scope)
	if len(scopes) == 0 || !contains(scopes, "openid") {
		return ValidatedAuthorization{},
			invalidRequest("scope must include openid", true)
	}
	for _, scope := range scopes {
		if !contains(app.Scopes, scope) {
			return ValidatedAuthorization{},
				invalidRequest(
					"requested scope is not allowed",
					true,
				)
		}
	}

	return ValidatedAuthorization{
		Application:   app,
		RedirectURI:   redirectURI,
		Scopes:        scopes,
		State:         state,
		Nonce:         nonce,
		CodeChallenge: request.CodeChallenge,
	}, nil
}

func (s *Service) IssueAuthorizationCode(
	ctx context.Context,
	authorization ValidatedAuthorization,
	user auth.User,
	authTime time.Time,
	authMethod string,
) (string, error) {
	if !user.Enabled {
		return "", fmt.Errorf("cannot authorize disabled identity")
	}

	rawCode, err := randomToken("mwac_")
	if err != nil {
		return "", err
	}
	codeID, err := randomToken("code_")
	if err != nil {
		return "", err
	}

	now := s.now().UTC()
	if authTime.IsZero() {
		authTime = now
	}

	err = s.store.CreateAuthorizationCode(
		ctx,
		AuthorizationCode{
			ID:            codeID,
			CodeHash:      hashOpaque(rawCode),
			ClientID:      authorization.Application.ClientID,
			UserID:        user.ID,
			RedirectURI:   authorization.RedirectURI,
			Scopes:        authorization.Scopes,
			CodeChallenge: authorization.CodeChallenge,
			Nonce:         authorization.Nonce,
			AuthTime:      authTime.UTC(),
			AuthMethod:    normalizedAuthMethod(authMethod),
			ExpiresAt:     now.Add(s.config.AuthorizationCodeTTL),
			CreatedAt:     now,
		},
	)
	if err != nil {
		return "", err
	}

	_ = s.store.DeleteExpiredOAuthArtifacts(ctx, now)
	return rawCode, nil
}

func (s *Service) ExchangeCode(
	ctx context.Context,
	request TokenRequest,
) (TokenResponse, *ProtocolError) {
	if request.GrantType != "authorization_code" {
		return TokenResponse{}, unsupportedGrantType()
	}

	app, err := s.store.ApplicationByClientID(
		ctx,
		strings.TrimSpace(request.ClientID),
	)
	if err != nil || !app.Enabled || app.ArchivedAt != nil {
		return TokenResponse{},
			invalidClient("client authentication failed")
	}

	switch app.ApplicationType {
	case "confidential":
		if request.ClientSecret == "" ||
			!applications.ClientSecretMatches(
				request.ClientSecret,
				app.ClientSecretHash,
			) {
			return TokenResponse{},
				invalidClient("client authentication failed")
		}
	case "public":
		// Public clients have no client secret. PKCE is mandatory.
	default:
		return TokenResponse{},
			invalidClient("client authentication failed")
	}

	if strings.TrimSpace(request.Code) == "" {
		return TokenResponse{},
			invalidGrant("authorization code is invalid or expired")
	}

	now := s.now().UTC()
	codeHash := hashOpaque(request.Code)

	code, err := s.store.AuthorizationCodeByHash(
		ctx,
		codeHash,
		now,
	)
	if err != nil {
		return TokenResponse{},
			invalidGrant("authorization code is invalid or expired")
	}

	if code.ClientID != app.ClientID ||
		code.RedirectURI != request.RedirectURI {
		return TokenResponse{},
			invalidGrant("authorization code binding mismatch")
	}

	if !ValidCodeVerifier(request.CodeVerifier) {
		return TokenResponse{},
			invalidGrant("invalid PKCE code_verifier")
	}

	actualChallenge := S256Challenge(request.CodeVerifier)
	if !constantTimeEqual(actualChallenge, code.CodeChallenge) {
		return TokenResponse{},
			invalidGrant("PKCE verification failed")
	}

	if err := s.store.ConsumeAuthorizationCode(
		ctx,
		codeHash,
		now,
	); err != nil {
		return TokenResponse{},
			invalidGrant("authorization code is invalid or expired")
	}

	user, err := s.store.IdentityUserByID(ctx, code.UserID)
	if err != nil || !user.Enabled {
		return TokenResponse{},
			invalidGrant("identity is unavailable")
	}

	accessToken, err := randomToken("mwat_")
	if err != nil {
		return TokenResponse{}, serverError("token generation failed")
	}
	accessID, err := randomToken("atk_")
	if err != nil {
		return TokenResponse{}, serverError("token generation failed")
	}

	idClaims := map[string]any{
		"iss":       s.config.Issuer,
		"sub":       user.ID,
		"aud":       app.ClientID,
		"exp":       now.Add(s.config.IDTokenTTL).Unix(),
		"iat":       now.Unix(),
		"auth_time": code.AuthTime.Unix(),
		"amr":       []string{code.AuthMethod},
	}
	if code.Nonce != "" {
		idClaims["nonce"] = code.Nonce
	}
	if contains(code.Scopes, "email") {
		idClaims["email"] = user.Email
	}
	if contains(code.Scopes, "profile") {
		idClaims["name"] = user.DisplayName
	}

	idToken, err := s.signing.SignJWT(ctx, idClaims)
	if err != nil {
		slog.Error(
			"OIDC ID token signing failed",
			"error", err,
		)
		return TokenResponse{}, serverError("ID token signing failed")
	}

	accessAudience := app.ClientID
	if contains(code.Scopes, "identity.admin") {
		if !contains(
			app.Audiences,
			IdentityAdminAudience,
		) {
			return TokenResponse{},
				serverError(
					"client is not configured for Identity admin audience",
				)
		}
		accessAudience = IdentityAdminAudience
	}

	accessExpires := now.Add(s.config.AccessTokenTTL)
	if err := s.store.CreateAccessToken(
		ctx,
		AccessToken{
			ID:            accessID,
			TokenHash:     hashOpaque(accessToken),
			ClientID:      app.ClientID,
			UserID:        user.ID,
			Audience:      accessAudience,
			Scopes:        code.Scopes,
			AuthTime:      code.AuthTime,
			AuthMethod:    normalizedAuthMethod(code.AuthMethod),
			ParentTokenID: "",
			ExpiresAt:     accessExpires,
			CreatedAt:     now,
		},
	); err != nil {
		slog.Error(
			"OIDC access token persistence failed",
			"error", err,
		)
		return TokenResponse{}, serverError("token persistence failed")
	}

	return TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.config.AccessTokenTTL.Seconds()),
		Scope:       strings.Join(code.Scopes, " "),
		IDToken:     idToken,
	}, nil
}

func (s *Service) UserInfo(
	ctx context.Context,
	rawAccessToken string,
) (map[string]any, *ProtocolError) {
	rawAccessToken = strings.TrimSpace(rawAccessToken)
	if rawAccessToken == "" {
		return nil, invalidToken()
	}

	token, err := s.store.AccessTokenByHash(
		ctx,
		hashOpaque(rawAccessToken),
		s.now().UTC(),
	)
	if err != nil {
		return nil, invalidToken()
	}

	user, err := s.store.IdentityUserByID(ctx, token.UserID)
	if err != nil || !user.Enabled {
		return nil, invalidToken()
	}

	claims := map[string]any{
		"sub": user.ID,
	}
	if contains(token.Scopes, "email") {
		claims["email"] = user.Email
	}
	if contains(token.Scopes, "profile") {
		claims["name"] = user.DisplayName
	}

	return claims, nil
}

func normalizedAuthMethod(value string) string {
	switch strings.TrimSpace(value) {
	case "password":
		return "password"
	case "webauthn":
		return "webauthn"
	default:
		return "unknown"
	}
}

func serverError(description string) *ProtocolError {
	return &ProtocolError{
		Code:        "server_error",
		Description: description,
		HTTPStatus:  500,
	}
}

func parseScope(value string) []string {
	fields := strings.Fields(value)
	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{})

	for _, field := range fields {
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		result = append(result, field)
	}

	return result
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func randomToken(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate OIDC token: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
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
