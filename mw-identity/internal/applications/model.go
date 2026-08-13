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
	ID                     string
	Name                   string
	ClientID               string
	ClientSecretHash       string
	ApplicationType        string
	AuthenticationPolicy   string
	Enabled                bool
	ArchivedAt             *time.Time
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	Scopes                 []string
	Claims                 []string
	Audiences              []string
	CreatedAt              time.Time
	UpdatedAt              time.Time
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
