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
	"strconv"
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

	acr := strings.TrimSpace(request.ACRValues)
	if acr == "" {
		acr = auth.ACRStandard
	}
	if acr != auth.ACRStandard && acr != auth.ACRStrong && acr != auth.ACRWebAuthn {
		return ValidatedAuthorization{}, invalidRequest("unsupported acr_values", true)
	}
	// Retain legacy app-wide Strong until the protected consumer is explicitly migrated.
	if app.AuthenticationPolicy == "strong" && acr == auth.ACRStandard {
		acr = auth.ACRStrong
	}
	requireWebAuthn := acr == auth.ACRWebAuthn
	var maxAgeSeconds int64 = 300
	if rawMaxAge := strings.TrimSpace(request.MaxAge); rawMaxAge != "" {
		parsed, err := strconv.ParseInt(rawMaxAge, 10, 64)
		if err != nil || parsed < 0 || parsed > 3600 {
			return ValidatedAuthorization{}, invalidRequest("invalid max_age", true)
		}
		maxAgeSeconds = parsed
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
		Application:     app,
		RequestedACR:    acr,
		RedirectURI:     redirectURI,
		Scopes:          scopes,
		State:           state,
		Nonce:           nonce,
		CodeChallenge:   request.CodeChallenge,
		RequireWebAuthn: requireWebAuthn,
		MaxAgeSeconds:   maxAgeSeconds,
		MaxAgeSet:       request.MaxAge != "",
	}, nil
}

func (s *Service) IssueAuthorizationCode(
	ctx context.Context,
	authorization ValidatedAuthorization,
	user auth.User,
	authTime time.Time,
	authMethod string, proof string,
) (string, error) {
	if !user.Enabled || user.DeletionRequestedAt != nil {
		return "", fmt.Errorf("cannot authorize disabled identity")
	}

	if authMethod != auth.MethodForProof(proof) || !auth.Satisfies(proof, authorization.RequestedACR) || authTime.IsZero() || authTime.After(s.now().UTC()) {
		return "", fmt.Errorf("authentication evidence required")
	}
	if authorization.MaxAgeSet && authorization.MaxAgeSeconds > 0 && s.now().Sub(authTime) > time.Duration(authorization.MaxAgeSeconds)*time.Second {
		return "", fmt.Errorf("authentication expired")
	}
	consent, err := s.Consent(ctx, authorization, user)
	if err != nil || !ConsentCovers(consent, authorization.Scopes) || (authorization.ConsentVersion != 0 && authorization.ConsentVersion != consent.Version) {
		return "", fmt.Errorf("consent required")
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

	maxAge := int64(-1)
	if authorization.MaxAgeSet {
		maxAge = authorization.MaxAgeSeconds
	}
	err = s.store.CreateAuthorizationCode(
		ctx,
		AuthorizationCode{
			ID:             codeID,
			ConsentVersion: consent.Version,
			Proof:          proof, RequestedACR: authorization.RequestedACR, MaxAgeSeconds: maxAge,
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

	if code.AuthMethod != auth.MethodForProof(code.Proof) || !auth.Satisfies(code.Proof, code.RequestedACR) || code.AuthTime.After(now) || code.AuthTime.Unix() <= 0 || (code.MaxAgeSeconds > 0 && now.Sub(code.AuthTime) > time.Duration(code.MaxAgeSeconds)*time.Second) {
		return TokenResponse{}, invalidGrant("authentication evidence invalid or expired")
	}
	if app.AuthenticationPolicy == "strong" && !auth.Satisfies(code.Proof, auth.ACRStrong) {
		return TokenResponse{}, invalidGrant("strong authentication required")
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

	accountType := normalizedAccountType(user.AccountType)
	achieved, methods := auth.Assurance(code.Proof)
	idClaims := map[string]any{
		"iss":             s.config.Issuer,
		"sub":             user.ID,
		"aud":             app.ClientID,
		"exp":             now.Add(s.config.IDTokenTTL).Unix(),
		"iat":             now.Unix(),
		"auth_time":       code.AuthTime.Unix(),
		"amr":             methods,
		"acr":             achieved,
		"mw_account_type": accountType,
	}
	if code.Nonce != "" {
		idClaims["nonce"] = code.Nonce
	}
	if contains(code.Scopes, "email") {
		idClaims["email"] = user.Email
		idClaims["email_verified"] = user.EmailVerifiedAt != nil
	}
	if contains(code.Scopes, "profile") {
		idClaims["name"] = user.DisplayName
		if user.Username != "" {
			idClaims["preferred_username"] = user.Username
		}
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
			ID:             accessID,
			ConsentVersion: code.ConsentVersion,
			TokenHash:      hashOpaque(accessToken),
			ClientID:       app.ClientID,
			UserID:         user.ID,
			Audience:       accessAudience,
			Scopes:         code.Scopes,
			AuthTime:       code.AuthTime,
			Proof:          code.Proof,
			AuthMethod:     normalizedAuthMethod(code.AuthMethod),
			ParentTokenID:  "",
			ExpiresAt:      accessExpires,
			CreatedAt:      now,
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

	accountType := normalizedAccountType(user.AccountType)
	claims := map[string]any{
		"sub":             user.ID,
		"mw_account_type": accountType,
	}
	if contains(token.Scopes, "email") {
		claims["email"] = user.Email
		claims["email_verified"] = user.EmailVerifiedAt != nil
	}
	if contains(token.Scopes, "profile") {
		claims["name"] = user.DisplayName
		if user.Username != "" {
			claims["preferred_username"] = user.Username
		}
	}

	return claims, nil
}

func normalizedAccountType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "workforce"
	}
	return value
}

func normalizedAuthMethod(value string) string {
	switch strings.TrimSpace(value) {
	case "password":
		return "password"
	case "password_totp":
		return "password_totp"
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
