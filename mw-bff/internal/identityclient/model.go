package identityclient

import (
	"context"
	"errors"
	"time"
)

var (
	ErrTransactionNotFound = errors.New(
		"OIDC login transaction not found",
	)
	ErrIdentityProtocol = errors.New(
		"Identity OIDC protocol failure",
	)
)

type Transaction struct {
	ID                     string
	StateHash              string
	NonceHash              string
	PKCEVerifierCiphertext []byte
	PKCEVerifierNonce      []byte
	EncryptionKeyVersion   int
	ReturnTo               string
	ExpiresAt              time.Time
	CreatedAt              time.Time
}

type Identity struct {
	Issuer               string
	Subject              string
	Email                string
	DisplayName          string
	AuthTime             time.Time
	AuthMethod           string
	AccessToken          string
	AccessTokenExpiresAt time.Time
}

type Store interface {
	CreateOIDCTransaction(
		context.Context,
		Transaction,
	) error

	ClaimOIDCTransaction(
		context.Context,
		string,
		time.Time,
	) (Transaction, error)

	DeleteExpiredOIDCTransactions(
		context.Context,
		time.Time,
	) error
}
