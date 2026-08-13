package main

import (
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrExpired           = errors.New("expired")
	ErrDuplicateSubject  = errors.New("external subject is already linked to another local account")
	ErrIssuerAlreadyLink = errors.New("local account already has a different subject for this issuer")
	ErrInvalidSession    = errors.New("invalid local session")
)

type LocalUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Session struct {
	TokenHash  string
	UserID     string
	AuthMethod string
	ExpiresAt  time.Time
}

type ExternalIdentity struct {
	LocalUserID string    `json:"local_user_id"`
	Issuer      string    `json:"issuer"`
	Subject     string    `json:"subject"`
	LinkedAt    time.Time `json:"linked_at"`
	LastLoginAt time.Time `json:"last_login_at,omitempty"`
}

type OIDCTransaction struct {
	Mode        string
	StateHash   string
	NonceHash   string
	Verifier    string
	LocalUserID string
	SessionHash string
	ExpiresAt   time.Time
}

type IdentityClaims struct {
	Issuer      string
	Subject     string
	Email       string
	DisplayName string
}

type PendingLink struct {
	TokenHash string
	Identity  IdentityClaims
	ExpiresAt time.Time
}

type AuditEvent struct {
	Type        string    `json:"type"`
	LocalUserID string    `json:"local_user_id,omitempty"`
	Issuer      string    `json:"issuer,omitempty"`
	Subject     string    `json:"subject,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
