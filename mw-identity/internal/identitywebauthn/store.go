package identitywebauthn

import (
	"context"
	"errors"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

var (
	ErrNotFound        = errors.New("WebAuthn resource not found")
	ErrConflict        = errors.New("WebAuthn resource conflict")
	ErrCeremonyUsed    = errors.New("WebAuthn ceremony already consumed")
	ErrCeremonyExpired = errors.New("WebAuthn ceremony expired")
)

type Store interface {
	IdentityUserByID(context.Context, string) (auth.User, error)

	EnsureIdentityWebAuthnUser(
		context.Context,
		string,
		string,
		[]byte,
	) (UserRecord, error)

	IdentityWebAuthnUser(
		context.Context,
		string,
		string,
	) (UserRecord, error)

	FindIdentityWebAuthnUserForCredential(
		context.Context,
		string,
		[]byte,
		[]byte,
	) (auth.User, UserRecord, error)

	ListIdentityWebAuthnCredentials(
		context.Context,
		string,
		string,
	) ([]CredentialRecord, error)

	InsertIdentityWebAuthnCredential(
		context.Context,
		CredentialRecord,
	) (CredentialRecord, error)

	UpdateIdentityWebAuthnCredential(
		context.Context,
		CredentialRecord,
		string,
	) error

	RevokeIdentityWebAuthnCredential(
		context.Context,
		string,
		string,
		[]byte,
		string,
	) error

	CreateIdentityWebAuthnCeremony(
		context.Context,
		CeremonyRecord,
	) error

	ConsumeIdentityWebAuthnCeremony(
		context.Context,
		string,
		int64,
	) (CeremonyRecord, error)

	DeleteExpiredIdentityWebAuthnCeremonies(
		context.Context,
		int64,
	) error

	RecordAuditEvent(context.Context, audit.Event) error
}
