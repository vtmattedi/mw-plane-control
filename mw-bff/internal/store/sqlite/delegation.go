package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/delegation"
)

func (s *Store) DelegatedToken(
	ctx context.Context,
	adminSessionID string,
	audience string,
	scope string,
	validAfter time.Time,
) (delegation.CacheRecord, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
            id, admin_session_id, audience, scope,
            token_ciphertext, token_nonce, encryption_key_version,
            expires_at_unix, created_at_unix, updated_at_unix
         FROM bff_delegated_tokens
         WHERE admin_session_id = ?
           AND audience = ?
           AND scope = ?
           AND expires_at_unix > ?`,
		adminSessionID,
		audience,
		scope,
		validAfter.Unix(),
	)

	var value delegation.CacheRecord
	var expires, created, updated int64
	err := row.Scan(
		&value.ID,
		&value.AdminSessionID,
		&value.Audience,
		&value.Scope,
		&value.TokenCiphertext,
		&value.TokenNonce,
		&value.EncryptionKeyVersion,
		&expires,
		&created,
		&updated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return delegation.CacheRecord{}, delegation.ErrNotFound
	}
	if err != nil {
		return delegation.CacheRecord{}, fmt.Errorf("load delegated token: %w", err)
	}
	value.ExpiresAt = time.Unix(expires, 0).UTC()
	value.CreatedAt = time.Unix(created, 0).UTC()
	value.UpdatedAt = time.Unix(updated, 0).UTC()
	return value, nil
}

func (s *Store) UpsertDelegatedToken(
	ctx context.Context,
	value delegation.CacheRecord,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO bff_delegated_tokens (
            id, admin_session_id, audience, scope,
            token_ciphertext, token_nonce, encryption_key_version,
            expires_at_unix, created_at_unix, updated_at_unix
         ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(admin_session_id, audience, scope) DO UPDATE SET
            id = excluded.id,
            token_ciphertext = excluded.token_ciphertext,
            token_nonce = excluded.token_nonce,
            encryption_key_version = excluded.encryption_key_version,
            expires_at_unix = excluded.expires_at_unix,
            updated_at_unix = excluded.updated_at_unix`,
		value.ID,
		value.AdminSessionID,
		value.Audience,
		value.Scope,
		value.TokenCiphertext,
		value.TokenNonce,
		value.EncryptionKeyVersion,
		value.ExpiresAt.Unix(),
		value.CreatedAt.Unix(),
		value.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("upsert delegated token: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredDelegatedTokens(
	ctx context.Context,
	now time.Time,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM bff_delegated_tokens WHERE expires_at_unix <= ?`,
		now.Unix(),
	)
	if err != nil {
		return fmt.Errorf("delete expired delegated tokens: %w", err)
	}
	return nil
}
