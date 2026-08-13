package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitywebauthn"
)

func (s *Store) EnsureIdentityWebAuthnUser(
	ctx context.Context,
	userID string,
	rpid string,
	userHandle []byte,
) (identitywebauthn.UserRecord, error) {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO identity_webauthn_users(
            user_id, rpid, user_handle
        ) VALUES (?, ?, ?)
        ON CONFLICT(user_id, rpid) DO NOTHING`,
		userID,
		rpid,
		userHandle,
	)
	if err != nil {
		return identitywebauthn.UserRecord{}, fmt.Errorf(
			"ensure Identity WebAuthn user: %w",
			err,
		)
	}

	return s.IdentityWebAuthnUser(ctx, userID, rpid)
}

func (s *Store) IdentityWebAuthnUser(
	ctx context.Context,
	userID string,
	rpid string,
) (identitywebauthn.UserRecord, error) {
	var record identitywebauthn.UserRecord

	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, rpid, user_handle, created_at
         FROM identity_webauthn_users
         WHERE user_id = ? AND rpid = ?
         LIMIT 1`,
		userID,
		rpid,
	).Scan(
		&record.ID,
		&record.UserID,
		&record.RPID,
		&record.UserHandle,
		&record.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return identitywebauthn.UserRecord{}, identitywebauthn.ErrNotFound
		}
		return identitywebauthn.UserRecord{}, fmt.Errorf(
			"load Identity WebAuthn user: %w",
			err,
		)
	}

	return record, nil
}

func (s *Store) FindIdentityWebAuthnUserForCredential(
	ctx context.Context,
	rpid string,
	userHandle []byte,
	credentialID []byte,
) (auth.User, identitywebauthn.UserRecord, error) {
	var record identitywebauthn.UserRecord
	var userID string

	err := s.db.QueryRowContext(
		ctx,
		`SELECT wu.id, wu.user_id, wu.rpid, wu.user_handle, wu.created_at
         FROM identity_webauthn_users AS wu
         JOIN identity_webauthn_credentials AS c
           ON c.user_id = wu.user_id
          AND c.rpid = wu.rpid
         WHERE wu.rpid = ?
           AND wu.user_handle = ?
           AND c.credential_id = ?
           AND c.revoked_at IS NULL
         LIMIT 1`,
		rpid,
		userHandle,
		credentialID,
	).Scan(
		&record.ID,
		&record.UserID,
		&record.RPID,
		&record.UserHandle,
		&record.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, identitywebauthn.UserRecord{}, identitywebauthn.ErrNotFound
		}
		return auth.User{}, identitywebauthn.UserRecord{}, fmt.Errorf(
			"find Identity WebAuthn user: %w",
			err,
		)
	}

	userID = record.UserID
	user, err := s.IdentityUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return auth.User{}, identitywebauthn.UserRecord{}, identitywebauthn.ErrNotFound
		}
		return auth.User{}, identitywebauthn.UserRecord{}, err
	}

	return user, record, nil
}

func (s *Store) ListIdentityWebAuthnCredentials(
	ctx context.Context,
	userID string,
	rpid string,
) ([]identitywebauthn.CredentialRecord, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
            id, user_id, rpid, credential_id,
            credential_ciphertext, credential_nonce,
            credential_key_version, label, transports_json,
            aaguid, sign_count, clone_warning,
            backup_eligible, backup_state, created_at,
            COALESCE(last_used_at, ''),
            COALESCE(revoked_at, '')
         FROM identity_webauthn_credentials
         WHERE user_id = ? AND rpid = ? AND revoked_at IS NULL
         ORDER BY created_at ASC, id ASC`,
		userID,
		rpid,
	)
	if err != nil {
		return nil, fmt.Errorf("list Identity WebAuthn credentials: %w", err)
	}
	defer rows.Close()

	records := make([]identitywebauthn.CredentialRecord, 0)
	for rows.Next() {
		var record identitywebauthn.CredentialRecord
		var signCount int64
		var cloneWarning, backupEligible, backupState int
		if err := rows.Scan(
			&record.ID,
			&record.UserID,
			&record.RPID,
			&record.CredentialID,
			&record.CredentialCiphertext,
			&record.CredentialNonce,
			&record.CredentialKeyVersion,
			&record.Label,
			&record.TransportsJSON,
			&record.AAGUID,
			&signCount,
			&cloneWarning,
			&backupEligible,
			&backupState,
			&record.CreatedAt,
			&record.LastUsedAt,
			&record.RevokedAt,
		); err != nil {
			return nil, fmt.Errorf("scan Identity WebAuthn credential: %w", err)
		}
		record.SignCount = uint32(signCount)
		record.CloneWarning = cloneWarning != 0
		record.BackupEligible = backupEligible != 0
		record.BackupState = backupState != 0
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Identity WebAuthn credentials: %w", err)
	}
	return records, nil
}

func (s *Store) InsertIdentityWebAuthnCredential(
	ctx context.Context,
	record identitywebauthn.CredentialRecord,
) (identitywebauthn.CredentialRecord, error) {
	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO identity_webauthn_credentials(
            user_id, rpid, credential_id,
            credential_ciphertext, credential_nonce,
            credential_key_version, label, transports_json,
            aaguid, sign_count, clone_warning,
            backup_eligible, backup_state
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.UserID,
		record.RPID,
		record.CredentialID,
		record.CredentialCiphertext,
		record.CredentialNonce,
		record.CredentialKeyVersion,
		record.Label,
		record.TransportsJSON,
		record.AAGUID,
		record.SignCount,
		boolInt(record.CloneWarning),
		boolInt(record.BackupEligible),
		boolInt(record.BackupState),
	)
	if err != nil {
		if identityWebAuthnUniqueConstraint(err) {
			return identitywebauthn.CredentialRecord{}, identitywebauthn.ErrConflict
		}
		return identitywebauthn.CredentialRecord{}, fmt.Errorf(
			"insert Identity WebAuthn credential: %w",
			err,
		)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return identitywebauthn.CredentialRecord{}, err
	}
	record.ID = id
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT created_at FROM identity_webauthn_credentials WHERE id = ?`,
		id,
	).Scan(&record.CreatedAt); err != nil {
		return identitywebauthn.CredentialRecord{}, err
	}
	return record, nil
}

func (s *Store) UpdateIdentityWebAuthnCredential(
	ctx context.Context,
	record identitywebauthn.CredentialRecord,
	now string,
) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE identity_webauthn_credentials
         SET credential_ciphertext = ?,
             credential_nonce = ?,
             credential_key_version = ?,
             transports_json = ?,
             aaguid = ?,
             sign_count = ?,
             clone_warning = ?,
             backup_eligible = ?,
             backup_state = ?,
             last_used_at = ?
         WHERE user_id = ? AND rpid = ? AND credential_id = ?
           AND revoked_at IS NULL`,
		record.CredentialCiphertext,
		record.CredentialNonce,
		record.CredentialKeyVersion,
		record.TransportsJSON,
		record.AAGUID,
		record.SignCount,
		boolInt(record.CloneWarning),
		boolInt(record.BackupEligible),
		boolInt(record.BackupState),
		now,
		record.UserID,
		record.RPID,
		record.CredentialID,
	)
	if err != nil {
		return fmt.Errorf("update Identity WebAuthn credential: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return identitywebauthn.ErrNotFound
	}
	return nil
}

func (s *Store) RevokeIdentityWebAuthnCredential(
	ctx context.Context,
	userID string,
	rpid string,
	credentialID []byte,
	now string,
) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE identity_webauthn_credentials
         SET revoked_at = ?
         WHERE user_id = ? AND rpid = ? AND credential_id = ?
           AND revoked_at IS NULL`,
		now,
		userID,
		rpid,
		credentialID,
	)
	if err != nil {
		return fmt.Errorf("revoke Identity WebAuthn credential: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return identitywebauthn.ErrNotFound
	}
	return nil
}

func (s *Store) CreateIdentityWebAuthnCeremony(
	ctx context.Context,
	record identitywebauthn.CeremonyRecord,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO identity_webauthn_ceremonies(
            token_hash, user_id, rpid, purpose,
            session_ciphertext, session_nonce, session_key_version,
            label, expires_at_unix
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.TokenHash,
		record.UserID,
		record.RPID,
		record.Purpose,
		record.SessionCiphertext,
		record.SessionNonce,
		record.SessionKeyVersion,
		record.Label,
		record.ExpiresAtUnix,
	)
	if err != nil {
		return fmt.Errorf("create Identity WebAuthn ceremony: %w", err)
	}
	return nil
}

func (s *Store) ConsumeIdentityWebAuthnCeremony(
	ctx context.Context,
	tokenHash string,
	nowUnix int64,
) (identitywebauthn.CeremonyRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identitywebauthn.CeremonyRecord{}, fmt.Errorf(
			"begin Identity WebAuthn ceremony transaction: %w",
			err,
		)
	}
	defer func() { _ = tx.Rollback() }()

	var record identitywebauthn.CeremonyRecord
	var userID sql.NullString
	var consumedAt sql.NullInt64

	err = tx.QueryRowContext(
		ctx,
		`SELECT
            id, token_hash, user_id, rpid, purpose,
            session_ciphertext, session_nonce, session_key_version,
            label, expires_at_unix, consumed_at_unix, created_at
         FROM identity_webauthn_ceremonies
         WHERE token_hash = ? LIMIT 1`,
		tokenHash,
	).Scan(
		&record.ID,
		&record.TokenHash,
		&userID,
		&record.RPID,
		&record.Purpose,
		&record.SessionCiphertext,
		&record.SessionNonce,
		&record.SessionKeyVersion,
		&record.Label,
		&record.ExpiresAtUnix,
		&consumedAt,
		&record.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return identitywebauthn.CeremonyRecord{}, identitywebauthn.ErrNotFound
		}
		return identitywebauthn.CeremonyRecord{}, err
	}

	if userID.Valid {
		value := userID.String
		record.UserID = &value
	}
	if consumedAt.Valid {
		value := consumedAt.Int64
		record.ConsumedAtUnix = &value
		return identitywebauthn.CeremonyRecord{}, identitywebauthn.ErrCeremonyUsed
	}
	if record.ExpiresAtUnix < nowUnix {
		return identitywebauthn.CeremonyRecord{}, identitywebauthn.ErrCeremonyExpired
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE identity_webauthn_ceremonies
         SET consumed_at_unix = ?
         WHERE id = ? AND consumed_at_unix IS NULL`,
		nowUnix,
		record.ID,
	)
	if err != nil {
		return identitywebauthn.CeremonyRecord{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return identitywebauthn.CeremonyRecord{}, err
	}
	if affected != 1 {
		return identitywebauthn.CeremonyRecord{}, identitywebauthn.ErrCeremonyUsed
	}
	if err := tx.Commit(); err != nil {
		return identitywebauthn.CeremonyRecord{}, err
	}
	value := nowUnix
	record.ConsumedAtUnix = &value
	return record, nil
}

func (s *Store) DeleteExpiredIdentityWebAuthnCeremonies(
	ctx context.Context,
	nowUnix int64,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM identity_webauthn_ceremonies
         WHERE expires_at_unix < ?`,
		nowUnix,
	)
	if err != nil {
		return fmt.Errorf("delete expired Identity WebAuthn ceremonies: %w", err)
	}
	return nil
}

func identityWebAuthnUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	value := strings.ToLower(err.Error())
	return strings.Contains(value, "unique constraint") ||
		strings.Contains(value, "constraint failed")
}
