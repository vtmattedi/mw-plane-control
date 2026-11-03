package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound                 = errors.New("identity record not found")
	ErrEmailExists              = errors.New("identity email already exists")
	ErrUsernameExists           = errors.New("identity username already exists")
	ErrInvalidLogin             = errors.New("invalid email or password")
	ErrUserDisabled             = errors.New("identity user is disabled")
	ErrInvalidSession           = errors.New("invalid or expired identity session")
	ErrRecoveryPending          = errors.New("identity recovery is pending")
	ErrAccountDeletionForbidden = errors.New("account deletion is forbidden")
)

type User struct {
	ID                  string
	Email               string
	EmailNormalized     string
	DisplayName         string
	Username            string
	UsernameNormalized  string
	AccountType         string
	EmailVerifiedAt     *time.Time
	DeletionRequestedAt *time.Time
	PasswordHash        string
	Enabled             bool
	IsSuperadmin        bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type NewUser struct {
	ID                 string
	Email              string
	EmailNormalized    string
	DisplayName        string
	Username           string
	UsernameNormalized string
	AccountType        string
	EmailVerifiedAt    *time.Time
	PasswordHash       string
	Enabled            bool
	IsSuperadmin       bool
	CreatedAt          time.Time
}

type SessionRecord struct {
	Proof      string
	ID         string
	UserID     string
	TokenHash  string
	CSRFHash   string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
	AuthMethod string
	AuthTime   time.Time
}

type Session struct {
	Proof      string
	ID         string
	User       User
	TokenHash  string
	CSRFHash   string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
	AuthMethod string
	AuthTime   time.Time
}

type ConnectedApplication struct {
	AccountManagementURI string    `json:"account_management_uri,omitempty"`
	AccountDeletionURI   string    `json:"account_deletion_uri,omitempty"`
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	ClientID             string    `json:"client_id"`
	Role                 string    `json:"role"`
	DeleteCallbackURI    string    `json:"-"`
	ConnectedAt          time.Time `json:"connected_at"`
}

type DeletionDelivery struct {
	ApplicationID string `json:"application_id"`
	ClientID      string `json:"client_id"`
	Status        string `json:"status"`
	Attempts      int    `json:"attempts"`
}

type DeleteAccountResult struct {
	OperationID string             `json:"operation_id,omitempty"`
	Status      string             `json:"status,omitempty"`
	StatusToken string             `json:"-"`
	Deleted     bool               `json:"deleted"`
	Deliveries  []DeletionDelivery `json:"deliveries"`
}

type SignupApplication struct {
	AccessPolicy      string
	ID                string
	Name              string
	ClientID          string
	AllowPublicSignup bool
	Enabled           bool
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

	IdentityUserRecoveryPending(
		context.Context,
		string,
	) (bool, error)

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

	ListIdentityUserApplications(context.Context, string) ([]ConnectedApplication, error)
	ListIdentityUserDeletionApplications(context.Context, string) ([]ConnectedApplication, error)
	BeginIdentityUserDeletion(context.Context, string, time.Time) error
	DeleteIdentityUser(context.Context, string) error
	RevokeIdentityUserApplicationAccess(context.Context, string, string) error
	PublicSignupApplication(context.Context, string) (SignupApplication, error)
	CreatePublicIdentityUser(context.Context, NewUser, string) error
}
