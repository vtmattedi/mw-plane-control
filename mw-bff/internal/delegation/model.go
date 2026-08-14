package delegation

import (
	"context"
	"time"
)

type CacheRecord struct {
	ID                   string
	AdminSessionID       string
	Audience             string
	Scope                string
	TokenCiphertext      []byte
	TokenNonce           []byte
	EncryptionKeyVersion int
	ExpiresAt            time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Store interface {
	DelegatedToken(
		context.Context,
		string,
		string,
		string,
		time.Time,
	) (CacheRecord, error)

	UpsertDelegatedToken(
		context.Context,
		CacheRecord,
	) error

	DeleteExpiredDelegatedTokens(
		context.Context,
		time.Time,
	) error
}

type Token struct {
	AccessToken string
	ExpiresAt   time.Time
	Cached      bool
}
