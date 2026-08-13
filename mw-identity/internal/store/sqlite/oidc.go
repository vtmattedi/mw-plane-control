package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
)

func (s *Store) CreateAuthorizationCode(
	ctx context.Context,
	code oidc.AuthorizationCode,
) error {
	scopeJSON, err := json.Marshal(code.Scopes)
	if err != nil {
		return fmt.Errorf("marshal authorization scopes: %w", err)
	}

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO identity_authorization_codes (
			id, code_hash, client_id, user_id, redirect_uri, scope_json,
			code_challenge, nonce, auth_time_unix, expires_at_unix,
			consumed_at_unix, created_at_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?)`,
		code.ID,
		code.CodeHash,
		code.ClientID,
		code.UserID,
		code.RedirectURI,
		string(scopeJSON),
		code.CodeChallenge,
		nullableString(code.Nonce),
		code.AuthTime.Unix(),
		code.ExpiresAt.Unix(),
		code.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("create authorization code: %w", err)
	}
	return nil
}

func (s *Store) AuthorizationCodeByHash(
	ctx context.Context,
	codeHash string,
	now time.Time,
) (oidc.AuthorizationCode, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			id, code_hash, client_id, user_id, redirect_uri, scope_json,
			code_challenge, nonce, auth_time_unix, expires_at_unix,
			consumed_at_unix, created_at_unix
		FROM identity_authorization_codes
		WHERE code_hash = ?
		  AND consumed_at_unix IS NULL
		  AND expires_at_unix > ?`,
		codeHash,
		now.Unix(),
	)

	var (
		code      oidc.AuthorizationCode
		scopeJSON string
		nonce     sql.NullString
		authTime  int64
		expires   int64
		consumed  sql.NullInt64
		created   int64
	)

	err := row.Scan(
		&code.ID,
		&code.CodeHash,
		&code.ClientID,
		&code.UserID,
		&code.RedirectURI,
		&scopeJSON,
		&code.CodeChallenge,
		&nonce,
		&authTime,
		&expires,
		&consumed,
		&created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return oidc.AuthorizationCode{}, oidc.ErrStoreNotFound
	}
	if err != nil {
		return oidc.AuthorizationCode{},
			fmt.Errorf("load authorization code: %w", err)
	}

	if err := json.Unmarshal([]byte(scopeJSON), &code.Scopes); err != nil {
		return oidc.AuthorizationCode{},
			fmt.Errorf("decode authorization scopes: %w", err)
	}

	code.Nonce = nonce.String
	code.AuthTime = time.Unix(authTime, 0).UTC()
	code.ExpiresAt = time.Unix(expires, 0).UTC()
	code.CreatedAt = time.Unix(created, 0).UTC()
	if consumed.Valid {
		value := time.Unix(consumed.Int64, 0).UTC()
		code.ConsumedAt = &value
	}

	return code, nil
}

func (s *Store) ConsumeAuthorizationCode(
	ctx context.Context,
	codeHash string,
	now time.Time,
) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE identity_authorization_codes
		 SET consumed_at_unix = ?
		 WHERE code_hash = ?
		   AND consumed_at_unix IS NULL
		   AND expires_at_unix > ?`,
		now.Unix(),
		codeHash,
		now.Unix(),
	)
	if err != nil {
		return fmt.Errorf("consume authorization code: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("authorization code rows affected: %w", err)
	}
	if affected != 1 {
		return oidc.ErrStoreNotFound
	}
	return nil
}

func (s *Store) CreateAccessToken(
	ctx context.Context,
	token oidc.AccessToken,
) error {
	scopeJSON, err := json.Marshal(token.Scopes)
	if err != nil {
		return fmt.Errorf("marshal access token scopes: %w", err)
	}

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO identity_access_tokens (
			id, token_hash, client_id, user_id, scope_json,
			expires_at_unix, created_at_unix, revoked_at_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
		token.ID,
		token.TokenHash,
		token.ClientID,
		token.UserID,
		string(scopeJSON),
		token.ExpiresAt.Unix(),
		token.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("create access token: %w", err)
	}
	return nil
}

func (s *Store) AccessTokenByHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (oidc.AccessToken, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			id, token_hash, client_id, user_id, scope_json,
			expires_at_unix, created_at_unix
		FROM identity_access_tokens
		WHERE token_hash = ?
		  AND revoked_at_unix IS NULL
		  AND expires_at_unix > ?`,
		tokenHash,
		now.Unix(),
	)

	var (
		token     oidc.AccessToken
		scopeJSON string
		expires   int64
		created   int64
	)

	err := row.Scan(
		&token.ID,
		&token.TokenHash,
		&token.ClientID,
		&token.UserID,
		&scopeJSON,
		&expires,
		&created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return oidc.AccessToken{}, oidc.ErrStoreNotFound
	}
	if err != nil {
		return oidc.AccessToken{},
			fmt.Errorf("load access token: %w", err)
	}

	if err := json.Unmarshal([]byte(scopeJSON), &token.Scopes); err != nil {
		return oidc.AccessToken{},
			fmt.Errorf("decode access token scopes: %w", err)
	}

	token.ExpiresAt = time.Unix(expires, 0).UTC()
	token.CreatedAt = time.Unix(created, 0).UTC()
	return token, nil
}

func (s *Store) DeleteExpiredOAuthArtifacts(
	ctx context.Context,
	now time.Time,
) error {
	if _, err := s.db.ExecContext(
		ctx,
		`DELETE FROM identity_authorization_codes
		 WHERE expires_at_unix <= ?
		    OR (
		        consumed_at_unix IS NOT NULL
		        AND consumed_at_unix <= ?
		    )`,
		now.Unix(),
		now.Add(-time.Hour).Unix(),
	); err != nil {
		return fmt.Errorf("delete authorization codes: %w", err)
	}

	if _, err := s.db.ExecContext(
		ctx,
		`DELETE FROM identity_access_tokens
		 WHERE expires_at_unix <= ?`,
		now.Unix(),
	); err != nil {
		return fmt.Errorf("delete access tokens: %w", err)
	}

	return nil
}
