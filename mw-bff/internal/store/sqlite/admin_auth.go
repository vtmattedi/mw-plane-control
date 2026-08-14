package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/identityclient"
	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/session"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateOIDCTransaction(
	ctx context.Context,
	value identityclient.Transaction,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO bff_oidc_transactions (
			id,
			state_hash,
			nonce_hash,
			pkce_verifier_ciphertext,
			pkce_verifier_nonce,
			encryption_key_version,
			return_to,
			expires_at_unix,
			consumed_at_unix,
			created_at_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?)`,
		value.ID,
		value.StateHash,
		value.NonceHash,
		value.PKCEVerifierCiphertext,
		value.PKCEVerifierNonce,
		value.EncryptionKeyVersion,
		value.ReturnTo,
		value.ExpiresAt.Unix(),
		value.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf(
			"create BFF OIDC transaction: %w",
			err,
		)
	}
	return nil
}

func (s *Store) ClaimOIDCTransaction(
	ctx context.Context,
	stateHash string,
	now time.Time,
) (identityclient.Transaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identityclient.Transaction{},
			fmt.Errorf(
				"begin BFF OIDC transaction claim: %w",
				err,
			)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	row := tx.QueryRowContext(
		ctx,
		`SELECT
			id,
			state_hash,
			nonce_hash,
			pkce_verifier_ciphertext,
			pkce_verifier_nonce,
			encryption_key_version,
			return_to,
			expires_at_unix,
			created_at_unix
		FROM bff_oidc_transactions
		WHERE state_hash = ?
		  AND consumed_at_unix IS NULL
		  AND expires_at_unix > ?`,
		stateHash,
		now.Unix(),
	)

	var (
		value   identityclient.Transaction
		expires int64
		created int64
	)

	err = row.Scan(
		&value.ID,
		&value.StateHash,
		&value.NonceHash,
		&value.PKCEVerifierCiphertext,
		&value.PKCEVerifierNonce,
		&value.EncryptionKeyVersion,
		&value.ReturnTo,
		&expires,
		&created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return identityclient.Transaction{},
			identityclient.ErrTransactionNotFound
	}
	if err != nil {
		return identityclient.Transaction{},
			fmt.Errorf(
				"load BFF OIDC transaction: %w",
				err,
			)
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE bff_oidc_transactions
		 SET consumed_at_unix = ?
		 WHERE id = ?
		   AND consumed_at_unix IS NULL
		   AND expires_at_unix > ?`,
		now.Unix(),
		value.ID,
		now.Unix(),
	)
	if err != nil {
		return identityclient.Transaction{},
			fmt.Errorf(
				"consume BFF OIDC transaction: %w",
				err,
			)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return identityclient.Transaction{},
			fmt.Errorf(
				"BFF OIDC claim rows affected: %w",
				err,
			)
	}
	if affected != 1 {
		return identityclient.Transaction{},
			identityclient.ErrTransactionNotFound
	}

	if err := tx.Commit(); err != nil {
		return identityclient.Transaction{},
			fmt.Errorf(
				"commit BFF OIDC transaction claim: %w",
				err,
			)
	}

	value.ExpiresAt = time.Unix(
		expires,
		0,
	).UTC()
	value.CreatedAt = time.Unix(
		created,
		0,
	).UTC()

	return value, nil
}

func (s *Store) DeleteExpiredOIDCTransactions(
	ctx context.Context,
	now time.Time,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM bff_oidc_transactions
		 WHERE expires_at_unix <= ?
		    OR (
		        consumed_at_unix IS NOT NULL
		        AND consumed_at_unix <= ?
		    )`,
		now.Unix(),
		now.Add(-time.Hour).Unix(),
	)
	if err != nil {
		return fmt.Errorf(
			"delete expired BFF OIDC transactions: %w",
			err,
		)
	}
	return nil
}

func (s *Store) CreateAdminSession(
	ctx context.Context,
	value session.Record,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO bff_admin_sessions (
			id,
			token_hash,
			csrf_hash,
			identity_issuer,
			identity_subject,
			email,
			display_name,
			auth_time_unix,
			auth_method,
			expires_at_unix,
			created_at_unix,
			last_seen_at_unix,
			identity_access_token_ciphertext,
			identity_access_token_nonce,
			identity_access_token_key_version
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID,
		value.TokenHash,
		value.CSRFHash,
		value.IdentityIssuer,
		value.IdentitySubject,
		value.Email,
		value.DisplayName,
		value.AuthTime.Unix(),
		value.AuthMethod,
		value.ExpiresAt.Unix(),
		value.CreatedAt.Unix(),
		value.LastSeenAt.Unix(),
		value.IdentityAccessTokenCiphertext,
		value.IdentityAccessTokenNonce,
		value.IdentityAccessTokenKeyVersion,
	)
	if err != nil {
		return fmt.Errorf(
			"create BFF admin session: %w",
			err,
		)
	}
	return nil
}

func (s *Store) AdminSessionByTokenHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (session.Record, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			id,
			token_hash,
			csrf_hash,
			identity_issuer,
			identity_subject,
			email,
			display_name,
			auth_time_unix,
			auth_method,
			expires_at_unix,
			created_at_unix,
			last_seen_at_unix,
			identity_access_token_ciphertext,
			identity_access_token_nonce,
			identity_access_token_key_version
		FROM bff_admin_sessions
		WHERE token_hash = ?
		  AND expires_at_unix > ?`,
		tokenHash,
		now.Unix(),
	)

	var (
		value    session.Record
		authTime int64
		expires  int64
		created  int64
		lastSeen int64
	)

	err := row.Scan(
		&value.ID,
		&value.TokenHash,
		&value.CSRFHash,
		&value.IdentityIssuer,
		&value.IdentitySubject,
		&value.Email,
		&value.DisplayName,
		&authTime,
		&value.AuthMethod,
		&expires,
		&created,
		&lastSeen,
		&value.IdentityAccessTokenCiphertext,
		&value.IdentityAccessTokenNonce,
		&value.IdentityAccessTokenKeyVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return session.Record{},
			session.ErrNotFound
	}
	if err != nil {
		return session.Record{},
			fmt.Errorf(
				"load BFF admin session: %w",
				err,
			)
	}

	value.AuthTime = time.Unix(
		authTime,
		0,
	).UTC()
	value.ExpiresAt = time.Unix(
		expires,
		0,
	).UTC()
	value.CreatedAt = time.Unix(
		created,
		0,
	).UTC()
	value.LastSeenAt = time.Unix(
		lastSeen,
		0,
	).UTC()

	_, _ = s.db.ExecContext(
		ctx,
		`UPDATE bff_admin_sessions
		 SET last_seen_at_unix = ?
		 WHERE token_hash = ?`,
		now.Unix(),
		tokenHash,
	)

	return value, nil
}

func (s *Store) UpdateAdminSessionCSRF(
	ctx context.Context,
	tokenHash string,
	csrfHash string,
) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE bff_admin_sessions
		 SET csrf_hash = ?
		 WHERE token_hash = ?`,
		csrfHash,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf(
			"update BFF admin CSRF: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"BFF admin CSRF rows affected: %w",
			err,
		)
	}
	if affected != 1 {
		return session.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAdminSession(
	ctx context.Context,
	tokenHash string,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM bff_admin_sessions
		 WHERE token_hash = ?`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf(
			"delete BFF admin session: %w",
			err,
		)
	}
	return nil
}

func (s *Store) DeleteExpiredAdminSessions(
	ctx context.Context,
	now time.Time,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM bff_admin_sessions
		 WHERE expires_at_unix <= ?`,
		now.Unix(),
	)
	if err != nil {
		return fmt.Errorf(
			"delete expired BFF admin sessions: %w",
			err,
		)
	}
	return nil
}
