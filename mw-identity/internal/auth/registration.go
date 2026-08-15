package auth

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

type RegisterPublicRequest struct {
	ClientID string
	Email    string
	Username string
	Password []byte
}

func NormalizeUsername(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !usernamePattern.MatchString(value) {
		return "", fmt.Errorf("username must contain 3 to 32 letters, numbers, dots, underscores, or hyphens")
	}
	return value, nil
}

func (s *Service) PublicSignupApplication(ctx context.Context, clientID string) (SignupApplication, error) {
	app, err := s.store.PublicSignupApplication(ctx, strings.TrimSpace(clientID))
	if err != nil {
		return SignupApplication{}, err
	}
	if !app.Enabled || !app.AllowPublicSignup {
		return SignupApplication{}, ErrAccountDeletionForbidden
	}
	return app, nil
}

func (s *Service) RegisterPublic(ctx context.Context, request RegisterPublicRequest, remoteIP string) (User, error) {
	app, err := s.PublicSignupApplication(ctx, request.ClientID)
	if err != nil {
		return User{}, err
	}
	email, err := NormalizeEmail(request.Email)
	if err != nil {
		return User{}, err
	}
	username, err := NormalizeUsername(request.Username)
	if err != nil {
		return User{}, err
	}
	passwordHash, err := HashPassword(request.Password)
	if err != nil {
		return User{}, err
	}
	userID, err := NewOpaqueToken("usr_")
	if err != nil {
		return User{}, err
	}
	now := s.now().UTC()
	input := NewUser{
		ID: userID, Email: strings.TrimSpace(request.Email), EmailNormalized: email,
		DisplayName: username, Username: username, UsernameNormalized: username,
		AccountType: "public", PasswordHash: passwordHash, Enabled: true, CreatedAt: now,
	}
	if err := s.store.CreatePublicIdentityUser(ctx, input, app.ID); err != nil {
		return User{}, err
	}
	user := User{
		ID: input.ID, Email: input.Email, EmailNormalized: input.EmailNormalized,
		DisplayName: input.DisplayName, Username: input.Username,
		UsernameNormalized: input.UsernameNormalized, AccountType: input.AccountType,
		PasswordHash: input.PasswordHash, Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	s.recordAudit(ctx, audit.Event{
		ID: mustAuditID(), EventType: "identity.user.created", Status: "success",
		Severity: "info", UserID: &user.ID, RemoteIP: remoteIP,
		ReasonCode: "public_registration", Metadata: map[string]any{"client_id": app.ClientID}, CreatedAt: now,
	})
	return user, nil
}
