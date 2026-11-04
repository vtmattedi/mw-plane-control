package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

func (s *Store) PublicSignupApplication(ctx context.Context, clientID string) (auth.SignupApplication, error) {
	var app auth.SignupApplication
	var enabled int
	var archived sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT id, name, client_id, enabled, archived_at_unix, access_policy
		FROM identity_applications WHERE client_id = ?`, clientID).Scan(
		&app.ID, &app.Name, &app.ClientID, &enabled, &archived, &app.AccessPolicy,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.SignupApplication{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.SignupApplication{}, fmt.Errorf("load public signup application: %w", err)
	}
	app.Enabled = enabled == 1 && !archived.Valid
	return app, nil
}

func (s *Store) CreatePublicIdentityUser(ctx context.Context, input auth.NewUser, applicationID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin public identity registration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_users (
		id,email,email_normalized,display_name,password_hash,enabled,is_superadmin,
		created_at_unix,updated_at_unix,username,username_normalized,account_type,email_verified_at_unix
	) VALUES (?, ?, ?, ?, ?, 1, 0, ?, ?, ?, ?, 'public', NULL)`,
		input.ID, input.Email, input.EmailNormalized, input.DisplayName, input.PasswordHash,
		input.CreatedAt.Unix(), input.CreatedAt.Unix(), input.Username, input.UsernameNormalized,
	)
	if err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "username") {
			return auth.ErrUsernameExists
		}
		if strings.Contains(message, "unique") {
			return auth.ErrEmailExists
		}
		return fmt.Errorf("create public identity user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit public identity registration: %w", err)
	}
	return nil
}
