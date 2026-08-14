package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
)

func (s *Store) IdentityUserRecoveryPending(ctx context.Context, userID string) (bool, error) {
	var pending int
	err := s.db.QueryRowContext(ctx,
		`SELECT recovery_pending FROM identity_users WHERE id = ?`, userID,
	).Scan(&pending)
	if errors.Is(err, sql.ErrNoRows) {
		return false, auth.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("load recovery state: %w", err)
	}
	return pending == 1, nil
}

func (s *Store) UpsertIdentityTOTP(ctx context.Context, record identitystrong.TOTPRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO identity_totp_credentials (
        user_id, secret_ciphertext, secret_nonce, secret_key_version,
        enabled, last_accepted_step, created_at_unix, updated_at_unix
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(user_id) DO UPDATE SET
        secret_ciphertext=excluded.secret_ciphertext,
        secret_nonce=excluded.secret_nonce,
        secret_key_version=excluded.secret_key_version,
        enabled=excluded.enabled,
        last_accepted_step=excluded.last_accepted_step,
        updated_at_unix=excluded.updated_at_unix`,
		record.UserID, record.SecretCiphertext, record.SecretNonce,
		record.SecretKeyVersion, boolStrong(record.Enabled), record.LastAcceptedStep,
		record.CreatedAt.Unix(), record.UpdatedAt.Unix())
	if err != nil {
		return fmt.Errorf("upsert Identity TOTP: %w", err)
	}
	return nil
}

func (s *Store) IdentityTOTPByUserID(ctx context.Context, userID string) (identitystrong.TOTPRecord, error) {
	var record identitystrong.TOTPRecord
	var enabled int
	var created, updated int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id, secret_ciphertext, secret_nonce,
        secret_key_version, enabled, last_accepted_step, created_at_unix, updated_at_unix
        FROM identity_totp_credentials WHERE user_id = ?`, userID).Scan(
		&record.UserID, &record.SecretCiphertext, &record.SecretNonce,
		&record.SecretKeyVersion, &enabled, &record.LastAcceptedStep, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return identitystrong.TOTPRecord{}, identitystrong.ErrNotFound
	}
	if err != nil {
		return identitystrong.TOTPRecord{}, fmt.Errorf("load Identity TOTP: %w", err)
	}
	record.Enabled = enabled == 1
	record.CreatedAt = time.Unix(created, 0).UTC()
	record.UpdatedAt = time.Unix(updated, 0).UTC()
	return record, nil
}

func (s *Store) AcceptIdentityTOTPStep(ctx context.Context, userID string, step int64, enable bool, now time.Time) error {
	query := `UPDATE identity_totp_credentials SET last_accepted_step = ?, updated_at_unix = ?
        WHERE user_id = ? AND last_accepted_step < ?`
	args := []any{step, now.Unix(), userID, step}
	if enable {
		query = `UPDATE identity_totp_credentials SET last_accepted_step = ?, enabled = 1, updated_at_unix = ?
            WHERE user_id = ? AND last_accepted_step < ?`
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("accept Identity TOTP step: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return identitystrong.ErrInvalidCode
	}
	return nil
}

func (s *Store) DeleteIdentityTOTP(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM identity_totp_credentials WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete Identity TOTP: %w", err)
	}
	return nil
}

func (s *Store) BeginIdentityRecovery(ctx context.Context, request identitystrong.RecoveryRequest, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `UPDATE identity_recovery_requests SET status='cancelled'
        WHERE user_id=? AND status='pending'`, request.UserID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO identity_recovery_requests (
        id,user_id,token_hash,status,expires_at_unix,consumed_at_unix,created_at_unix
        ) VALUES (?, ?, ?, 'pending', ?, NULL, ?)`, request.ID, request.UserID,
		request.TokenHash, request.ExpiresAt.Unix(), request.CreatedAt.Unix()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE identity_users SET recovery_pending=1 WHERE id=?`, request.UserID); err != nil {
		return err
	}
	if err = revokeIdentityTx(ctx, tx, request.UserID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) IdentityRecoveryByTokenHash(ctx context.Context, tokenHash string, now time.Time) (identitystrong.RecoveryRequest, auth.User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT r.id,r.user_id,r.token_hash,r.status,r.expires_at_unix,
        r.consumed_at_unix,r.created_at_unix,
        u.id,u.email,u.email_normalized,u.display_name,u.password_hash,u.enabled,u.is_superadmin,
        u.created_at_unix,u.updated_at_unix,u.recovery_pending
        FROM identity_recovery_requests r JOIN identity_users u ON u.id=r.user_id
        WHERE r.token_hash=? LIMIT 1`, tokenHash)
	var rec identitystrong.RecoveryRequest
	var user auth.User
	var expires, created int64
	var consumed sql.NullInt64
	var enabled, superadmin, pending int
	var userCreated, userUpdated int64
	err := row.Scan(&rec.ID, &rec.UserID, &rec.TokenHash, &rec.Status, &expires, &consumed, &created,
		&user.ID, &user.Email, &user.EmailNormalized, &user.DisplayName, &user.PasswordHash, &enabled, &superadmin,
		&userCreated, &userUpdated, &pending)
	if errors.Is(err, sql.ErrNoRows) {
		return rec, user, identitystrong.ErrRecovery
	}
	if err != nil {
		return rec, user, fmt.Errorf("load Identity recovery: %w", err)
	}
	rec.ExpiresAt = time.Unix(expires, 0).UTC()
	rec.CreatedAt = time.Unix(created, 0).UTC()
	if consumed.Valid {
		v := time.Unix(consumed.Int64, 0).UTC()
		rec.ConsumedAt = &v
	}
	user.Enabled = enabled == 1
	user.IsSuperadmin = superadmin == 1
	user.CreatedAt = time.Unix(userCreated, 0).UTC()
	user.UpdatedAt = time.Unix(userUpdated, 0).UTC()
	if pending != 1 || rec.Status != "pending" || !rec.ExpiresAt.After(now) {
		return identitystrong.RecoveryRequest{}, auth.User{}, identitystrong.ErrRecovery
	}
	return rec, user, nil
}

func (s *Store) ConsumeIdentityRecovery(ctx context.Context, requestID, userID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE identity_recovery_requests SET status='consumed', consumed_at_unix=?
        WHERE id=? AND user_id=? AND status='pending' AND expires_at_unix>?`, now.Unix(), requestID, userID, now.Unix())
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return identitystrong.ErrRecovery
	}
	if _, err = tx.ExecContext(ctx, `UPDATE identity_users SET recovery_pending=0 WHERE id=?`, userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity_totp_credentials WHERE user_id=?`, userID); err != nil {
		return err
	}
	if err = revokeIdentityTx(ctx, tx, userID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CancelIdentityRecovery(ctx context.Context, userID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `UPDATE identity_recovery_requests SET status='cancelled' WHERE user_id=? AND status='pending'`, userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE identity_users SET recovery_pending=0 WHERE id=?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RevokeIdentitySessionsAndTokens(ctx context.Context, userID string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = revokeIdentityTx(ctx, tx, userID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func revokeIdentityTx(ctx context.Context, tx *sql.Tx, userID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM identity_sessions WHERE user_id=?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM identity_authorization_codes WHERE user_id=?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_access_tokens SET revoked_at_unix=? WHERE user_id=? AND revoked_at_unix IS NULL`, now.Unix(), userID); err != nil {
		return err
	}
	return nil
}

func boolStrong(v bool) int {
	if v {
		return 1
	}
	return 0
}
