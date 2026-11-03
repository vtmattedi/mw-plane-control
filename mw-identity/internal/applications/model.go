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

// CheckLifecycle enforces the deletion contract: every application that is not
// internal must accept identity.user.deleted events. Internal applications run
// behind the MW BFF for the workforce, so they cannot open public sign-up.
func CheckLifecycle(internal, allowPublicSignup bool, deleteCallbackURI string) error {
	if internal {
		if allowPublicSignup {
			return ErrInvalidApplication
		}
		return nil
	}
	if deleteCallbackURI == "" {
		return ErrDeleteCallbackRequired
	}
	return nil
}

type Application struct {
	AccessPolicyV1         bool       `json:"access_policy_v1"`
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	ClientID               string     `json:"client_id"`
	ClientSecretHash       string     `json:"-"`
	ApplicationType        string     `json:"application_type"`
	AuthenticationPolicy   string     `json:"authentication_policy"`
	AccessPolicy           string     `json:"access_policy"`
	Enabled                bool       `json:"enabled"`
	ArchivedAt             *time.Time `json:"archived_at,omitempty"`
	RedirectURIs           []string   `json:"redirect_uris"`
	PostLogoutRedirectURIs []string   `json:"post_logout_redirect_uris"`
	DeleteCallbackURI      string     `json:"delete_callback_uri,omitempty"`
	AllowPublicSignup      bool       `json:"allow_public_signup"`
	Internal               bool       `json:"internal"`
	// NeedsDeleteCallback flags an application registered before the callback
	// became mandatory. It keeps working; its next edit must add one.
	NeedsDeleteCallback bool      `json:"needs_delete_callback"`
	Scopes              []string  `json:"scopes"`
	Claims              []string  `json:"claims"`
	Audiences           []string  `json:"audiences"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreateInput struct {
	AccessPolicyV1         bool
	ID                     string
	Name                   string
	ClientID               string
	ClientSecretHash       string
	ApplicationType        string
	AuthenticationPolicy   string
	AccessPolicy           string
	Enabled                bool
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	DeleteCallbackURI      string
	AllowPublicSignup      bool
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
