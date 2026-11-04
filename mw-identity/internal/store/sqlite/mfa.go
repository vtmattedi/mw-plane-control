package sqlite

import (
	"context"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"time"
)

func (s *Store) CreateMFAChallenge(ctx context.Context, c auth.MFAChallenge) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO identity_mfa_challenges(token_hash,binding_hash,user_id,password_fingerprint,return_to,expires_at_unix) VALUES (?,?,?,?,?,?)`, c.TokenHash, c.BindingHash, c.UserID, c.PasswordFingerprint, c.ReturnTo, c.ExpiresAt.Unix())
	return err
}
func (s *Store) AttemptMFAChallenge(ctx context.Context, token, binding string, now time.Time) (auth.MFAChallenge, error) {
	var c auth.MFAChallenge
	var expiry int64
	err := s.db.QueryRowContext(ctx, `UPDATE identity_mfa_challenges SET attempts=attempts+1 WHERE token_hash=? AND binding_hash=? AND consumed=0 AND attempts<5 AND expires_at_unix>? RETURNING token_hash,binding_hash,user_id,password_fingerprint,return_to,expires_at_unix`, token, binding, now.Unix()).Scan(&c.TokenHash, &c.BindingHash, &c.UserID, &c.PasswordFingerprint, &c.ReturnTo, &expiry)
	c.ExpiresAt = time.Unix(expiry, 0).UTC()
	return c, err
}
func (s *Store) CompleteMFAChallenge(ctx context.Context, token, binding string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE identity_mfa_challenges SET consumed=1 WHERE token_hash=? AND binding_hash=? AND consumed=0 AND expires_at_unix>?`, token, binding, now.Unix())
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return auth.ErrInvalidLogin
	}
	return nil
}
