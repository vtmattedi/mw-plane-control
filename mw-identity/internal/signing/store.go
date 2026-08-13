package signing

import (
	"context"
	"errors"
	"time"
)

var ErrNoActiveKey = errors.New("no active signing key")

type StoredKey struct {
	KID                  string
	Alg                  string
	Status               string
	PublicJWKJSON        string
	PrivateCiphertext    []byte
	PrivateNonce         []byte
	EncryptionKeyVersion int
	CreatedAt            time.Time
	RetiredAt            *time.Time
}

type Store interface {
	ActiveSigningKey(context.Context) (StoredKey, error)
	SigningKeyByKID(context.Context, string) (StoredKey, error)
	RotateSigningKey(context.Context, StoredKey, time.Time) error
	PublicSigningKeys(context.Context) ([]StoredKey, error)
}
