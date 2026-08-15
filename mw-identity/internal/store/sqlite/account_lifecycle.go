package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

func (s *Store) ListIdentityUserApplications(ctx context.Context, userID string) ([]auth.ConnectedApplication, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		p.id,
		p.name,
		p.client_id,
		a.role,
		COALESCE(p.delete_callback_uri, ''),
		a.created_at_unix
	FROM identity_application_assignments a
	JOIN identity_applications p ON p.id = a.application_id
	WHERE a.user_id = ?
	ORDER BY p.name ASC, p.id ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list connected applications: %w", err)
	}
	defer rows.Close()

	var result []auth.ConnectedApplication
	for rows.Next() {
		var app auth.ConnectedApplication
		var connectedAt int64
		if err := rows.Scan(&app.ID, &app.Name, &app.ClientID, &app.Role, &app.DeleteCallbackURI, &connectedAt); err != nil {
			return nil, fmt.Errorf("scan connected application: %w", err)
		}
		app.ConnectedAt = time.Unix(connectedAt, 0).UTC()
		result = append(result, app)
	}
	return result, rows.Err()
}

func (s *Store) BeginIdentityUserDeletion(ctx context.Context, userID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin identity user deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `UPDATE identity_users
		SET enabled = 0,
		    deletion_requested_at_unix = COALESCE(deletion_requested_at_unix, ?),
		    updated_at_unix = ?
		WHERE id = ?`, now.Unix(), now.Unix(), userID)
	if err != nil {
		return fmt.Errorf("disable deleting identity user: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return auth.ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity_sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("revoke deleting user sessions: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity_authorization_codes WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("revoke deleting user codes: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE identity_access_tokens
		SET revoked_at_unix = ?
		WHERE user_id = ? AND revoked_at_unix IS NULL`, now.Unix(), userID); err != nil {
		return fmt.Errorf("revoke deleting user tokens: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit identity user deletion: %w", err)
	}
	return nil
}

func (s *Store) DeleteIdentityUser(ctx context.Context, userID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM identity_users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete identity user: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return auth.ErrNotFound
	}
	return nil
}
