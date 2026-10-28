package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

func (s *Store) ListIdentityUserApplications(ctx context.Context, userID string) ([]auth.ConnectedApplication, error) {
	return s.listConsentApplications(ctx, userID, true)
}
func (s *Store) ListIdentityUserDeletionApplications(ctx context.Context, userID string) ([]auth.ConnectedApplication, error) {
	return s.listConsentApplications(ctx, userID, false)
}
func (s *Store) listConsentApplications(ctx context.Context, userID string, active bool) ([]auth.ConnectedApplication, error) {
	query := `SELECT p.id,p.name,p.client_id,'user',COALESCE(p.delete_callback_uri,''),c.created_at_unix FROM identity_applications p JOIN identity_application_consents c ON c.application_id=p.id WHERE c.user_id=? AND c.active=1 ORDER BY p.name,p.id`
	args := []any{userID}
	if !active {
		query = `WITH known AS (SELECT application_id,created_at_unix FROM identity_application_consents WHERE user_id=? UNION ALL SELECT application_id,created_at_unix FROM identity_application_assignments WHERE user_id=?) SELECT p.id,p.name,p.client_id,'user',COALESCE(p.delete_callback_uri,''),MIN(k.created_at_unix) FROM identity_applications p JOIN known k ON k.application_id=p.id GROUP BY p.id ORDER BY p.name,p.id`
		args = append(args, userID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []auth.ConnectedApplication
	for rows.Next() {
		var a auth.ConnectedApplication
		var created int64
		if err := rows.Scan(&a.ID, &a.Name, &a.ClientID, &a.Role, &a.DeleteCallbackURI, &created); err != nil {
			return nil, err
		}
		a.ConnectedAt = time.Unix(created, 0).UTC()
		result = append(result, a)
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

// RevokeIdentityUserApplicationAccess ends one application's access for a user.
// Consent history survives token cleanup and remains a whole-account deletion target.
func (s *Store) RevokeIdentityUserApplicationAccess(ctx context.Context, userID, clientID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin application access revocation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `INSERT INTO identity_application_consents (user_id,application_id,active,version,created_at_unix,updated_at_unix) SELECT ?,id,0,1,?,? FROM identity_applications WHERE client_id=? ON CONFLICT(user_id,application_id) DO UPDATE SET active=0,version=CASE WHEN active=1 THEN version+1 ELSE version END,updated_at_unix=excluded.updated_at_unix`, userID, time.Now().Unix(), time.Now().Unix(), clientID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity_authorization_codes
		WHERE user_id = ? AND client_id = ?`, userID, clientID); err != nil {
		return fmt.Errorf("revoke application authorization codes: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity_access_tokens
		WHERE user_id = ? AND client_id = ?`, userID, clientID); err != nil {
		return fmt.Errorf("revoke application access tokens: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit application access revocation: %w", err)
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
