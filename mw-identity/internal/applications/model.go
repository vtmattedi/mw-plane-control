package applications

import (
	"context"
	"errors"
	"time"
)

var (
	ErrClientIDExists      = errors.New("client id already exists")
	ErrApplicationNotFound = errors.New("application not found")
	ErrInvalidApplication  = errors.New("invalid application")
)

type Application struct {
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	ClientID               string     `json:"client_id"`
	ClientSecretHash       string     `json:"-"`
	ApplicationType        string     `json:"application_type"`
	AuthenticationPolicy   string     `json:"authentication_policy"`
	Enabled                bool       `json:"enabled"`
	ArchivedAt             *time.Time `json:"archived_at,omitempty"`
	RedirectURIs           []string   `json:"redirect_uris"`
	PostLogoutRedirectURIs []string   `json:"post_logout_redirect_uris"`
	DeleteCallbackURI      string     `json:"delete_callback_uri,omitempty"`
	AllowPublicSignup      bool       `json:"allow_public_signup"`
	Scopes                 []string   `json:"scopes"`
	Claims                 []string   `json:"claims"`
	Audiences              []string   `json:"audiences"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type CreateInput struct {
	ID                     string
	Name                   string
	ClientID               string
	ClientSecretHash       string
	ApplicationType        string
	AuthenticationPolicy   string
	Enabled                bool
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	DeleteCallbackURI      string
	AllowPublicSignup      bool
	Scopes                 []string
	Claims                 []string
	Audiences              []string
	CreatedAt              time.Time
}

type Store interface {
	CreateApplication(context.Context, CreateInput) error
	ApplicationByClientID(
		context.Context,
		string,
	) (Application, error)
}
