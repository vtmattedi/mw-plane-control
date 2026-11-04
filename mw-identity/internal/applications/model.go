package applications

import (
	"context"
	"errors"
	"time"
)

var (
	ErrClientIDExists         = errors.New("client id already exists")
	ErrApplicationNotFound    = errors.New("application not found")
	ErrInvalidApplication     = errors.New("invalid application")
	ErrDeleteCallbackRequired = errors.New("non-internal applications must configure a delete callback")
)

// CheckLifecycle requires ordinary applications to accept whole-identity deletion events.
func CheckLifecycle(internal bool, deleteCallbackURI string) error {
	if !internal && deleteCallbackURI == "" {
		return ErrDeleteCallbackRequired
	}
	return nil
}

type Application struct {
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	ClientID               string     `json:"client_id"`
	ClientSecretHash       string     `json:"-"`
	ApplicationType        string     `json:"application_type"`
	AccessPolicy           string     `json:"access_policy"`
	Enabled                bool       `json:"enabled"`
	ArchivedAt             *time.Time `json:"archived_at,omitempty"`
	RedirectURIs           []string   `json:"redirect_uris"`
	PostLogoutRedirectURIs []string   `json:"post_logout_redirect_uris"`
	DeleteCallbackURI      string     `json:"delete_callback_uri,omitempty"`
	AccountManagementURI   string     `json:"account_management_uri"`
	AccountDeletionURI     string     `json:"account_deletion_uri"`
	Internal               bool       `json:"internal"`
	// NeedsDeleteCallback flags an application registered before the callback
	// became mandatory. The registration must be completed before integration.
	NeedsDeleteCallback bool      `json:"needs_delete_callback"`
	Scopes              []string  `json:"scopes"`
	Claims              []string  `json:"claims"`
	Audiences           []string  `json:"audiences"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreateInput struct {
	ID                     string
	Name                   string
	ClientID               string
	ClientSecretHash       string
	ApplicationType        string
	AccessPolicy           string
	Enabled                bool
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	DeleteCallbackURI      string
	AccountManagementURI   string
	AccountDeletionURI     string
	Internal               bool
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
