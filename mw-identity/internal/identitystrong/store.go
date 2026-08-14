package identitystrong

import (
	"context"
	"errors"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

var (
	ErrNotFound           = errors.New("strong-auth record not found")
	ErrInvalidCode        = errors.New("invalid authenticator code")
	ErrTOTPDisabled       = errors.New("TOTP is not enabled")
	ErrTOTPAlreadyEnabled = errors.New("TOTP is already enabled")
	ErrRecovery           = errors.New("invalid recovery capability")
	ErrRecoveryExpiry     = errors.New("recovery capability expired")
)

type TOTPRecord struct {
	UserID           string
	SecretCiphertext string
	SecretNonce      string
	SecretKeyVersion int
	Enabled          bool
	LastAcceptedStep int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type RecoveryRequest struct {
	ID         string
	UserID     string
	TokenHash  string
	Status     string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

type Store interface {
	IdentityUserByEmail(context.Context, string) (auth.User, error)
	IdentityUserByID(context.Context, string) (auth.User, error)
	IdentityUserRecoveryPending(context.Context, string) (bool, error)

	UpsertIdentityTOTP(context.Context, TOTPRecord) error
	IdentityTOTPByUserID(context.Context, string) (TOTPRecord, error)
	AcceptIdentityTOTPStep(context.Context, string, int64, bool, time.Time) error
	DeleteIdentityTOTP(context.Context, string) error

	BeginIdentityRecovery(context.Context, RecoveryRequest, time.Time) error
	IdentityRecoveryByTokenHash(context.Context, string, time.Time) (RecoveryRequest, auth.User, error)
	ConsumeIdentityRecovery(context.Context, string, string, time.Time) error
	CancelIdentityRecovery(context.Context, string, time.Time) error
	RevokeIdentitySessionsAndTokens(context.Context, string, time.Time) error

	RecordAuditEvent(context.Context, audit.Event) error
}
