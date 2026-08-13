package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
)

func (s *Store) ActiveSigningKey(
	ctx context.Context,
) (signing.StoredKey, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			kid,
			alg,
			status,
			public_jwk_json,
			private_key_ciphertext,
			private_key_nonce,
			encryption_key_version,
			created_at_unix,
			retired_at_unix
		FROM identity_signing_keys
		WHERE status = 'active'
		LIMIT 1`,
	)

	return scanSigningKey(row)
}

func (s *Store) SigningKeyByKID(
	ctx context.Context,
	kid string,
) (signing.StoredKey, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			kid,
			alg,
			status,
			public_jwk_json,
			private_key_ciphertext,
			private_key_nonce,
			encryption_key_version,
			created_at_unix,
			retired_at_unix
		FROM identity_signing_keys
		WHERE kid = ?`,
		kid,
	)

	return scanSigningKey(row)
}

func (s *Store) RotateSigningKey(
	ctx context.Context,
	key signing.StoredKey,
	retiredAt time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf(
			"begin signing-key rotation: %w",
			err,
		)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE identity_signing_keys
		 SET status = 'retired',
		     retired_at_unix = ?
		 WHERE status = 'active'`,
		retiredAt.Unix(),
	); err != nil {
		return fmt.Errorf(
			"retire active signing key: %w",
			err,
		)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO identity_signing_keys (
			kid,
			alg,
			status,
			public_jwk_json,
			private_key_ciphertext,
			private_key_nonce,
			encryption_key_version,
			created_at_unix,
			retired_at_unix
		) VALUES (?, ?, 'active', ?, ?, ?, ?, ?, NULL)`,
		key.KID,
		key.Alg,
		key.PublicJWKJSON,
		key.PrivateCiphertext,
		key.PrivateNonce,
		key.EncryptionKeyVersion,
		key.CreatedAt.Unix(),
	); err != nil {
		return fmt.Errorf(
			"insert active signing key: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"commit signing-key rotation: %w",
			err,
		)
	}

	return nil
}

func (s *Store) PublicSigningKeys(
	ctx context.Context,
) ([]signing.StoredKey, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
			kid,
			alg,
			status,
			public_jwk_json,
			private_key_ciphertext,
			private_key_nonce,
			encryption_key_version,
			created_at_unix,
			retired_at_unix
		FROM identity_signing_keys
		ORDER BY created_at_unix ASC, kid ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list public signing keys: %w",
			err,
		)
	}
	defer rows.Close()

	var result []signing.StoredKey
	for rows.Next() {
		key, err := scanSigningKey(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate public signing keys: %w",
			err,
		)
	}

	return result, nil
}

func scanSigningKey(
	row rowScanner,
) (signing.StoredKey, error) {
	var (
		key     signing.StoredKey
		created int64
		retired sql.NullInt64
	)

	err := row.Scan(
		&key.KID,
		&key.Alg,
		&key.Status,
		&key.PublicJWKJSON,
		&key.PrivateCiphertext,
		&key.PrivateNonce,
		&key.EncryptionKeyVersion,
		&created,
		&retired,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return signing.StoredKey{}, signing.ErrNoActiveKey
	}
	if err != nil {
		return signing.StoredKey{}, fmt.Errorf(
			"scan signing key: %w",
			err,
		)
	}

	key.CreatedAt = time.Unix(created, 0).UTC()
	if retired.Valid {
		value := time.Unix(retired.Int64, 0).UTC()
		key.RetiredAt = &value
	}

	return key, nil
}
