package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("identity record not found")
	ErrEmailExists    = errors.New("identity email already exists")
	ErrInvalidLogin   = errors.New("invalid email or password")
	ErrUserDisabled   = errors.New("identity user is disabled")
	ErrInvalidSession = errors.New("invalid or expired identity session")
)

type User struct {
	ID              string
	Email           string
	EmailNormalized string
	DisplayName     string
	PasswordHash    string
	Enabled         bool
	IsSuperadmin    bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type NewUser struct {
	ID              string
	Email           string
	EmailNormalized string
	DisplayName     string
	PasswordHash    string
	Enabled         bool
	IsSuperadmin    bool
	CreatedAt       time.Time
}

type SessionRecord struct {
	ID         string
	UserID     string
	TokenHash  string
	CSRFHash   string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
}

type Session struct {
	ID         string
	User       User
	TokenHash  string
	CSRFHash   string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
}

type Store interface {
	CreateIdentityUser(context.Context, NewUser) error
	IdentityUserByEmail(
		context.Context,
		string,
	) (User, error)

	IdentityUserByID(
		context.Context,
		string,
	) (User, error)

	CreateIdentitySession(
		context.Context,
		SessionRecord,
	) error

	IdentitySessionByTokenHash(
		context.Context,
		string,
		time.Time,
	) (Session, error)

	UpdateIdentitySessionCSRF(
		context.Context,
		string,
		string,
	) error

	DeleteIdentitySessionByTokenHash(
		context.Context,
		string,
	) error

	DeleteExpiredIdentitySessions(
		context.Context,
		time.Time,
	) error
}
