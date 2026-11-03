package identityadmin

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

var (
	ErrNotFound         = errors.New("Identity admin record not found")
	ErrInvalid          = errors.New("invalid Identity admin request")
	ErrArchived         = errors.New("application is archived")
	ErrArchiveRequired  = errors.New("application must be archived before deletion")
	ErrApplicationInUse = errors.New("application has dependent identity data")
	ErrRoleForbidden    = errors.New("role is not permitted")
)

const (
	PermissionApplicationsRead   = "identity.applications.read"
	PermissionApplicationsWrite  = "identity.applications.write"
	PermissionApplicationsRotate = "identity.applications.rotate_secret"
	PermissionUsersRead          = "identity.users.read"
	PermissionUsersWrite         = "identity.users.write"
	PermissionAssignmentsRead    = "identity.assignments.read"
	PermissionAssignmentsWrite   = "identity.assignments.write"
	PermissionAuditRead          = "identity.audit.read"
)

type UpdateApplication struct {
	Name                   string   `json:"name"`
	AuthenticationPolicy   string   `json:"authentication_policy"`
	AccessPolicy           string   `json:"access_policy"`
	RedirectURIs           []string `json:"redirect_uris"`
	PostLogoutRedirectURIs []string `json:"post_logout_redirect_uris"`
	DeleteCallbackURI      string   `json:"delete_callback_uri"`
	AccountManagementURI   string   `json:"account_management_uri"`
	AccountDeletionURI     string   `json:"account_deletion_uri"`
	AllowPublicSignup      bool     `json:"allow_public_signup"`
	Internal               bool     `json:"internal"`
	Scopes                 []string `json:"scopes"`
	Claims                 []string `json:"claims"`
	Audiences              []string `json:"audiences"`
}

type UserSummary struct {
	ID                  string     `json:"id"`
	Email               string     `json:"email"`
	DisplayName         string     `json:"display_name"`
	Enabled             bool       `json:"enabled"`
	IsSuperadmin        bool       `json:"is_superadmin"`
	AccountType         string     `json:"account_type"`
	Username            string     `json:"username,omitempty"`
	EmailVerifiedAt     *time.Time `json:"email_verified_at"`
	DeletionRequestedAt *time.Time `json:"deletion_requested_at"`
	CreatedAt           time.Time  `json:"created_at"`
}

type CreateUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type Assignment struct {
	ApplicationID string    `json:"application_id"`
	UserID        string    `json:"user_id"`
	Role          string    `json:"role"`
	AccessEffect  string    `json:"access_effect"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AuditEntry struct {
	ID           string          `json:"id"`
	EventType    string          `json:"event_type"`
	Status       string          `json:"status"`
	Severity     string          `json:"severity"`
	UserID       *string         `json:"user_id,omitempty"`
	RemoteIP     string          `json:"remote_ip,omitempty"`
	ReasonCode   string          `json:"reason_code,omitempty"`
	MetadataJSON json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
}

type Store interface {
	// Existing application registry operations used by applications.Service.
	CreateApplication(
		context.Context,
		applications.CreateInput,
	) error

	ApplicationByClientID(
		context.Context,
		string,
	) (applications.Application, error)

	// Identity Admin operations.
	ListIdentityApplications(
		context.Context,
	) ([]applications.Application, error)

	IdentityApplicationByID(
		context.Context,
		string,
	) (applications.Application, error)

	UpdateIdentityApplication(
		context.Context,
		string,
		UpdateApplication,
		time.Time,
	) error

	SetIdentityApplicationState(
		context.Context,
		string,
		bool,
		*time.Time,
		time.Time,
	) error

	PurgeIdentityApplication(
		context.Context,
		string,
		string,
	) error

	UpdateIdentityApplicationSecretHash(
		context.Context,
		string,
		string,
		time.Time,
	) error

	ListIdentityUsers(
		context.Context,
	) ([]UserSummary, error)

	CreateIdentityUser(context.Context, auth.NewUser) error

	ListIdentityAssignments(
		context.Context,
		string,
	) ([]Assignment, error)

	UpsertIdentityAssignment(
		context.Context,
		Assignment,
	) error

	DeleteIdentityAssignment(
		context.Context,
		string,
		string,
	) error

	IdentityAssignmentRole(
		context.Context,
		string,
		string,
	) (string, error)

	IdentityAssignment(
		context.Context,
		string,
		string,
	) (Assignment, error)

	IdentityAssignmentRoleByClientID(
		context.Context,
		string,
		string,
	) (string, error)

	ListIdentityAudit(
		context.Context,
		int,
	) ([]AuditEntry, error)

	RecordAuditEvent(
		context.Context,
		audit.Event,
	) error

	IdentityUserByID(
		context.Context,
		string,
	) (auth.User, error)
}
