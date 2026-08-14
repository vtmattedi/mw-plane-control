package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/secure"
)

var (
	ErrNotFound = errors.New(
		"admin session not found",
	)
	ErrInvalid = errors.New(
		"invalid or expired admin session",
	)
)

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

type Record struct {
	ID                            string
	TokenHash                     string
	CSRFHash                      string
	IdentityIssuer                string
	IdentitySubject               string
	Email                         string
	DisplayName                   string
	AuthTime                      time.Time
	AuthMethod                    string
	IdentityAccessTokenCiphertext []byte
	IdentityAccessTokenNonce      []byte
	IdentityAccessTokenKeyVersion int
	ExpiresAt                     time.Time
	CreatedAt                     time.Time
	LastSeenAt                    time.Time
}

type Store interface {
	CreateAdminSession(
		context.Context,
		Record,
	) error

	AdminSessionByTokenHash(
		context.Context,
		string,
		time.Time,
	) (Record, error)

	UpdateAdminSessionCSRF(
		context.Context,
		string,
		string,
	) error

	DeleteAdminSession(
		context.Context,
		string,
	) error

	DeleteExpiredAdminSessions(
		context.Context,
		time.Time,
	) error
}

type Service struct {
	store   Store
	ttl     time.Duration
	keyring secure.Keyring
	now     func() time.Time
}

type Created struct {
	RawSession string
	RawCSRF    string
	Record     Record
}

func New(
	store Store,
	ttl time.Duration,
	keyring secure.Keyring,
) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf(
			"admin session store is required",
		)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf(
			"admin session TTL must be positive",
		)
	}

	return &Service{
		store:   store,
		ttl:     ttl,
		keyring: keyring,
		now:     time.Now,
	}, nil
}

func (s *Service) Create(
	ctx context.Context,
	identity Identity,
) (Created, error) {
	if strings.TrimSpace(identity.Issuer) == "" ||
		strings.TrimSpace(identity.Subject) == "" {
		return Created{}, fmt.Errorf(
			"identity issuer/subject required",
		)
	}

	rawSession, err := randomToken("mwadmin_")
	if err != nil {
		return Created{}, err
	}
	rawCSRF, err := randomToken("mwcsrf_")
	if err != nil {
		return Created{}, err
	}
	id, err := randomToken("ads_")
	if err != nil {
		return Created{}, err
	}

	if strings.TrimSpace(identity.AccessToken) == "" {
		return Created{}, fmt.Errorf(
			"Identity access token is required",
		)
	}

	accessBytes := []byte(identity.AccessToken)
	defer func() {
		for i := range accessBytes {
			accessBytes[i] = 0
		}
	}()

	accessCiphertext, accessNonce, accessKeyVersion, err :=
		s.keyring.Encrypt(
			"mw-bff-session-access-token:"+id,
			accessBytes,
		)
	if err != nil {
		return Created{}, err
	}

	now := s.now().UTC()
	expiresAt := now.Add(s.ttl)
	if !identity.AccessTokenExpiresAt.IsZero() &&
		identity.AccessTokenExpiresAt.Before(expiresAt) {
		expiresAt = identity.AccessTokenExpiresAt.UTC()
	}
	if !expiresAt.After(now) {
		return Created{}, fmt.Errorf(
			"Identity access token is already expired",
		)
	}

	record := Record{
		ID:                            id,
		TokenHash:                     HashToken(rawSession),
		CSRFHash:                      HashToken(rawCSRF),
		IdentityIssuer:                identity.Issuer,
		IdentitySubject:               identity.Subject,
		Email:                         identity.Email,
		DisplayName:                   identity.DisplayName,
		AuthTime:                      identity.AuthTime.UTC(),
		AuthMethod:                    strings.TrimSpace(identity.AuthMethod),
		IdentityAccessTokenCiphertext: accessCiphertext,
		IdentityAccessTokenNonce:      accessNonce,
		IdentityAccessTokenKeyVersion: accessKeyVersion,
		ExpiresAt:                     expiresAt,
		CreatedAt:                     now,
		LastSeenAt:                    now,
	}

	if err := s.store.CreateAdminSession(
		ctx,
		record,
	); err != nil {
		return Created{}, err
	}

	_ = s.store.DeleteExpiredAdminSessions(
		ctx,
		now,
	)

	return Created{
		RawSession: rawSession,
		RawCSRF:    rawCSRF,
		Record:     record,
	}, nil
}

func (s *Service) Get(
	ctx context.Context,
	rawSession string,
) (Record, error) {
	if strings.TrimSpace(rawSession) == "" {
		return Record{}, ErrInvalid
	}

	record, err := s.store.AdminSessionByTokenHash(
		ctx,
		HashToken(rawSession),
		s.now().UTC(),
	)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Record{}, ErrInvalid
		}
		return Record{}, err
	}

	return record, nil
}

func (s *Service) RotateCSRF(
	ctx context.Context,
	rawSession string,
) (string, error) {
	rawCSRF, err := randomToken("mwcsrf_")
	if err != nil {
		return "", err
	}

	if err := s.store.UpdateAdminSessionCSRF(
		ctx,
		HashToken(rawSession),
		HashToken(rawCSRF),
	); err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrInvalid
		}
		return "", err
	}

	return rawCSRF, nil
}

func (s *Service) Logout(
	ctx context.Context,
	rawSession string,
) error {
	if strings.TrimSpace(rawSession) == "" {
		return nil
	}

	return s.store.DeleteAdminSession(
		ctx,
		HashToken(rawSession),
	)
}

func (s *Service) IdentityAccessToken(
	record Record,
) (string, error) {
	if len(record.IdentityAccessTokenCiphertext) == 0 ||
		len(record.IdentityAccessTokenNonce) == 0 ||
		record.IdentityAccessTokenKeyVersion <= 0 {
		return "", ErrInvalid
	}

	plaintext, err := s.keyring.Decrypt(
		record.IdentityAccessTokenKeyVersion,
		"mw-bff-session-access-token:"+record.ID,
		record.IdentityAccessTokenNonce,
		record.IdentityAccessTokenCiphertext,
	)
	if err != nil {
		return "", err
	}
	defer func() {
		for i := range plaintext {
			plaintext[i] = 0
		}
	}()

	return string(plaintext), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func TokenMatchesHash(
	raw string,
	expectedHash string,
) bool {
	actual := HashToken(raw)
	if len(actual) != len(expectedHash) {
		return false
	}
	return subtle.ConstantTimeCompare(
		[]byte(actual),
		[]byte(expectedHash),
	) == 1
}

func TokensEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare(
		[]byte(left),
		[]byte(right),
	) == 1
}

func randomToken(prefix string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf(
			"generate session token: %w",
			err,
		)
	}

	return prefix +
		base64.RawURLEncoding.EncodeToString(raw), nil
}
