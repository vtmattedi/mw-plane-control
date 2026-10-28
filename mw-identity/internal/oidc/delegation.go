package oidc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
)

const (
	TokenExchangeGrantType = "urn:ietf:params:oauth:grant-type:token-exchange"
	AccessTokenType        = "urn:ietf:params:oauth:token-type:access_token"
)

type TokenExchangeRequest struct {
	GrantType          string
	ClientID           string
	ClientSecret       string
	SubjectToken       string
	SubjectTokenType   string
	RequestedTokenType string
	Audience           string
	Scope              string
}

type IntrospectionResponse struct {
	Active      bool     `json:"active"`
	Subject     string   `json:"sub,omitempty"`
	Audience    string   `json:"aud,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	ClientID    string   `json:"client_id,omitempty"`
	ExpiresAt   int64    `json:"exp,omitempty"`
	IssuedAt    int64    `json:"iat,omitempty"`
	AuthTime    int64    `json:"auth_time,omitempty"`
	AuthMethod  []string `json:"amr,omitempty"`
	AccountType string   `json:"mw_account_type,omitempty"`
}

func (s *Service) ExchangeToken(
	ctx context.Context,
	request TokenExchangeRequest,
) (TokenResponse, *ProtocolError) {
	if request.GrantType != TokenExchangeGrantType {
		return TokenResponse{}, unsupportedGrantType()
	}
	if request.SubjectTokenType != AccessTokenType {
		return TokenResponse{}, invalidRequest(
			"subject_token_type must be access_token",
			false,
		)
	}
	if request.RequestedTokenType != "" &&
		request.RequestedTokenType != AccessTokenType {
		return TokenResponse{}, invalidRequest(
			"requested_token_type must be access_token",
			false,
		)
	}

	app, protocolErr := s.authenticateConfidentialClient(
		ctx,
		request.ClientID,
		request.ClientSecret,
	)
	if protocolErr != nil {
		return TokenResponse{}, protocolErr
	}

	rawSubject := strings.TrimSpace(request.SubjectToken)
	if rawSubject == "" {
		return TokenResponse{}, invalidGrant(
			"subject_token is invalid or expired",
		)
	}

	now := s.now().UTC()
	subject, err := s.store.AccessTokenByHash(
		ctx,
		hashOpaque(rawSubject),
		now,
	)
	if err != nil {
		return TokenResponse{}, invalidGrant(
			"subject_token is invalid or expired",
		)
	}
	if subject.ClientID != app.ClientID {
		return TokenResponse{}, invalidGrant(
			"subject_token is not bound to this client",
		)
	}

	user, err := s.store.IdentityUserByID(ctx, subject.UserID)
	if err != nil || !user.Enabled {
		return TokenResponse{}, invalidGrant(
			"subject identity is unavailable",
		)
	}

	audience := strings.TrimSpace(request.Audience)
	if audience == "" ||
		audience == IdentityAdminAudience ||
		!contains(app.Audiences, audience) {
		return TokenResponse{}, invalidTarget(
			"requested audience is not allowed",
		)
	}

	scopes := parseScope(request.Scope)
	if len(scopes) == 0 {
		return TokenResponse{}, invalidScope(
			"delegated scope is required",
		)
	}
	for _, scope := range scopes {
		if isIdentityOnlyScope(scope) ||
			!contains(app.Scopes, scope) ||
			!contains(subject.Scopes, scope) {
			return TokenResponse{}, invalidScope(
				"requested delegated scope is not allowed",
			)
		}
	}

	rawToken, err := randomToken("mwdt_")
	if err != nil {
		return TokenResponse{}, serverError(
			"delegated token generation failed",
		)
	}
	tokenID, err := randomToken("dtk_")
	if err != nil {
		return TokenResponse{}, serverError(
			"delegated token generation failed",
		)
	}

	expiresAt := now.Add(s.config.DelegatedTokenTTL)
	if subject.ExpiresAt.Before(expiresAt) {
		expiresAt = subject.ExpiresAt
	}
	if !expiresAt.After(now) {
		return TokenResponse{}, invalidGrant(
			"subject_token is invalid or expired",
		)
	}

	authMethod := strings.TrimSpace(subject.AuthMethod)
	if authMethod == "" {
		authMethod = "unknown"
	}

	if err := s.store.CreateAccessToken(
		ctx,
		AccessToken{
			ID:             tokenID,
			TokenHash:      hashOpaque(rawToken),
			ClientID:       app.ClientID,
			UserID:         subject.UserID,
			Audience:       audience,
			Scopes:         scopes,
			AuthTime:       subject.AuthTime,
			AuthMethod:     authMethod,
			ParentTokenID:  subject.ID,
			ConsentVersion: subject.ConsentVersion,
			ExpiresAt:      expiresAt,
			CreatedAt:      now,
		},
	); err != nil {
		return TokenResponse{}, serverError(
			"delegated token persistence failed",
		)
	}

	s.recordDelegationAudit(
		ctx,
		user.ID,
		app.ClientID,
		audience,
		scopes,
		authMethod,
		now,
	)

	_ = s.store.DeleteExpiredOAuthArtifacts(ctx, now)

	return TokenResponse{
		AccessToken:     rawToken,
		IssuedTokenType: AccessTokenType,
		TokenType:       "Bearer",
		ExpiresIn:       int64(expiresAt.Sub(now).Seconds()),
		Scope:           strings.Join(scopes, " "),
	}, nil
}

func (s *Service) Introspect(
	ctx context.Context,
	clientID string,
	clientSecret string,
	rawToken string,
) (IntrospectionResponse, *ProtocolError) {
	app, protocolErr := s.authenticateConfidentialClient(
		ctx,
		clientID,
		clientSecret,
	)
	if protocolErr != nil {
		return IntrospectionResponse{}, protocolErr
	}

	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return IntrospectionResponse{Active: false}, nil
	}

	token, err := s.store.AccessTokenByHash(
		ctx,
		hashOpaque(rawToken),
		s.now().UTC(),
	)
	if errors.Is(err, ErrStoreNotFound) {
		return IntrospectionResponse{Active: false}, nil
	}
	if err != nil {
		return IntrospectionResponse{}, serverError(
			"token introspection failed",
		)
	}

	// A resource server may only introspect tokens minted for itself.
	if token.Audience != app.ClientID {
		return IntrospectionResponse{Active: false}, nil
	}

	sourceApp, err := s.store.ApplicationByClientID(ctx, token.ClientID)
	if err != nil || !sourceApp.Enabled || sourceApp.ArchivedAt != nil {
		return IntrospectionResponse{Active: false}, nil
	}

	user, err := s.store.IdentityUserByID(ctx, token.UserID)
	if err != nil || !user.Enabled {
		return IntrospectionResponse{Active: false}, nil
	}

	response := IntrospectionResponse{
		Active:      true,
		Subject:     token.UserID,
		Audience:    token.Audience,
		Scope:       strings.Join(token.Scopes, " "),
		ClientID:    token.ClientID,
		ExpiresAt:   token.ExpiresAt.Unix(),
		IssuedAt:    token.CreatedAt.Unix(),
		AccountType: normalizedAccountType(user.AccountType),
	}
	if !token.AuthTime.IsZero() && token.AuthTime.Unix() > 0 {
		response.AuthTime = token.AuthTime.Unix()
	}
	if method := strings.TrimSpace(token.AuthMethod); method != "" {
		response.AuthMethod = []string{method}
	}
	return response, nil
}

func (s *Service) authenticateConfidentialClient(
	ctx context.Context,
	clientID string,
	clientSecret string,
) (applications.Application, *ProtocolError) {
	app, err := s.store.ApplicationByClientID(
		ctx,
		strings.TrimSpace(clientID),
	)
	if err != nil || !app.Enabled || app.ArchivedAt != nil {
		return applications.Application{}, invalidClient(
			"client authentication failed",
		)
	}
	if app.ApplicationType != "confidential" ||
		clientSecret == "" ||
		!applications.ClientSecretMatches(
			clientSecret,
			app.ClientSecretHash,
		) {
		return applications.Application{}, invalidClient(
			"client authentication failed",
		)
	}
	return app, nil
}

func isIdentityOnlyScope(scope string) bool {
	switch scope {
	case "openid", "profile", "email", "identity.admin":
		return true
	default:
		return false
	}
}

func (s *Service) recordDelegationAudit(
	ctx context.Context,
	userID string,
	clientID string,
	audience string,
	scopes []string,
	authMethod string,
	now time.Time,
) {
	recorder, ok := s.store.(interface {
		RecordAuditEvent(context.Context, audit.Event) error
	})
	if !ok {
		return
	}
	auditID, err := randomToken("aud_")
	if err != nil {
		return
	}
	_ = recorder.RecordAuditEvent(
		ctx,
		audit.Event{
			ID:         auditID,
			EventType:  "identity.token.delegated",
			Status:     "success",
			Severity:   "info",
			UserID:     &userID,
			ReasonCode: "token_exchange",
			Metadata: map[string]any{
				"client_id":   clientID,
				"audience":    audience,
				"scopes":      scopes,
				"auth_method": authMethod,
			},
			CreatedAt: now,
		},
	)
}
