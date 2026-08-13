package signing

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"
)

type memorySigningStore struct {
	mu     sync.Mutex
	active *StoredKey
	keys   []StoredKey
}

func (s *memorySigningStore) ActiveSigningKey(
	context.Context,
) (StoredKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active == nil {
		return StoredKey{}, ErrNoActiveKey
	}

	return cloneStoredKey(*s.active), nil
}

func (s *memorySigningStore) SigningKeyByKID(
	_ context.Context,
	kid string,
) (StoredKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range s.keys {
		if key.KID == kid {
			return cloneStoredKey(key), nil
		}
	}

	return StoredKey{}, ErrNoActiveKey
}

func (s *memorySigningStore) RotateSigningKey(
	_ context.Context,
	key StoredKey,
	retiredAt time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.keys {
		if s.keys[index].Status == "active" {
			s.keys[index].Status = "retired"
			value := retiredAt
			s.keys[index].RetiredAt = &value
		}
	}

	if s.active != nil {
		for index := range s.keys {
			if s.keys[index].KID == s.active.KID {
				s.keys[index].Status = "retired"
				value := retiredAt
				s.keys[index].RetiredAt = &value
			}
		}
	}

	copyKey := cloneStoredKey(key)
	s.keys = append(s.keys, copyKey)
	active := cloneStoredKey(copyKey)
	s.active = &active
	return nil
}

func (s *memorySigningStore) PublicSigningKeys(
	context.Context,
) ([]StoredKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]StoredKey, 0, len(s.keys))
	for _, key := range s.keys {
		result = append(result, cloneStoredKey(key))
	}
	return result, nil
}

func cloneStoredKey(value StoredKey) StoredKey {
	value.PrivateCiphertext = append(
		[]byte(nil),
		value.PrivateCiphertext...,
	)
	value.PrivateNonce = append(
		[]byte(nil),
		value.PrivateNonce...,
	)
	return value
}

func TestEnsureActiveAndRotatePreservesPublicKeys(
	t *testing.T,
) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = 7
	}

	keyring, err := ParseKeyring(
		"1:"+
			base64.RawURLEncoding.EncodeToString(raw),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	store := &memorySigningStore{}
	manager, err := NewManager(store, keyring)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	firstKID, err := manager.EnsureActiveKey(ctx)
	if err != nil {
		t.Fatal(err)
	}

	signer, err := manager.ActiveSigner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if signer.KID != firstKID {
		t.Fatalf(
			"signer kid = %q; want %q",
			signer.KID,
			firstKID,
		)
	}

	secondKID, err := manager.Rotate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if secondKID == firstKID {
		t.Fatal("rotation reused kid")
	}

	jwks, err := manager.PublicJWKS(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 2 {
		t.Fatalf(
			"JWKS key count = %d; want 2",
			len(jwks.Keys),
		)
	}
}

func TestActiveSignerRejectsMissingEncryptionVersion(
	t *testing.T,
) {
	raw := make([]byte, 32)
	keyring, err := ParseKeyring(
		"1:"+
			base64.RawURLEncoding.EncodeToString(raw),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	store := &memorySigningStore{
		active: &StoredKey{
			KID:                  "sig_test",
			EncryptionKeyVersion: 99,
		},
	}

	manager, err := NewManager(store, keyring)
	if err != nil {
		t.Fatal(err)
	}

	_, err = manager.ActiveSigner(context.Background())
	if err == nil {
		t.Fatal("expected unavailable encryption version error")
	}
	if errors.Is(err, ErrNoActiveKey) {
		t.Fatalf("unexpected error: %v", err)
	}
}
