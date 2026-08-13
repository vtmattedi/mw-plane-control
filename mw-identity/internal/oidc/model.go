package oidc

import (
	"context"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

type AuthorizationRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type ValidatedAuthorization struct {
	Application   applications.Application
	RedirectURI   string
	Scopes        []string
	State         string
	Nonce         string
	CodeChallenge string
}

type AuthorizationCode struct {
	ID            string
	CodeHash      string
	ClientID      string
	UserID        string
	RedirectURI   string
	Scopes        []string
	CodeChallenge string
	Nonce         string
	AuthTime      time.Time
	ExpiresAt     time.Time
	ConsumedAt    *time.Time
	CreatedAt     time.Time
}

type TokenRequest struct {
	GrantType    string
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string
	CodeVerifier string
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
	IDToken     string `json:"id_token"`
}

type AccessToken struct {
	ID        string
	TokenHash string
	ClientID  string
	UserID    string
	Audience  string
	Scopes    []string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Store interface {
	ApplicationByClientID(
		context.Context,
		string,
	) (applications.Application, error)

	IdentityAssignmentRole(
		context.Context,
		string,
		string,
	) (string, error)

	IdentityUserByID(
		context.Context,
		string,
	) (auth.User, error)

	CreateAuthorizationCode(
		context.Context,
		AuthorizationCode,
	) error

	AuthorizationCodeByHash(
		context.Context,
		string,
		time.Time,
	) (AuthorizationCode, error)

	ConsumeAuthorizationCode(
		context.Context,
		string,
		time.Time,
	) error

	CreateAccessToken(
		context.Context,
		AccessToken,
	) error

	AccessTokenByHash(
		context.Context,
		string,
		time.Time,
	) (AccessToken, error)

	DeleteExpiredOAuthArtifacts(
		context.Context,
		time.Time,
	) error
}
