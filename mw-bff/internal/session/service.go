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
	Issuer      string
	Subject     string
	Email       string
	DisplayName string
	AuthTime    time.Time
}

type Record struct {
	ID              string
	TokenHash       string
	CSRFHash        string
	IdentityIssuer  string
	IdentitySubject string
	Email           string
	DisplayName     string
	AuthTime        time.Time
	ExpiresAt       time.Time
	CreatedAt       time.Time
	LastSeenAt      time.Time
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
	store Store
	ttl   time.Duration
	now   func() time.Time
}

type Created struct {
	RawSession string
	RawCSRF    string
	Record     Record
}

func New(
	store Store,
	ttl time.Duration,
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
		store: store,
		ttl:   ttl,
		now:   time.Now,
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

	now := s.now().UTC()
	record := Record{
		ID:              id,
		TokenHash:       HashToken(rawSession),
		CSRFHash:        HashToken(rawCSRF),
		IdentityIssuer:  identity.Issuer,
		IdentitySubject: identity.Subject,
		Email:           identity.Email,
		DisplayName:     identity.DisplayName,
		AuthTime:        identity.AuthTime.UTC(),
		ExpiresAt:       now.Add(s.ttl),
		CreatedAt:       now,
		LastSeenAt:      now,
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
