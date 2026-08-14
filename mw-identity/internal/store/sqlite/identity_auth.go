package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateIdentityUser(
	ctx context.Context,
	input auth.NewUser,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO identity_users (
			id,
			email,
			email_normalized,
			display_name,
			password_hash,
			enabled,
			is_superadmin,
			created_at_unix,
			updated_at_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ID,
		input.Email,
		input.EmailNormalized,
		input.DisplayName,
		input.PasswordHash,
		boolInt(input.Enabled),
		boolInt(input.IsSuperadmin),
		input.CreatedAt.Unix(),
		input.CreatedAt.Unix(),
	)
	if err != nil {
		if strings.Contains(
			strings.ToLower(err.Error()),
			"unique",
		) {
			return auth.ErrEmailExists
		}
		return fmt.Errorf("create identity user: %w", err)
	}

	return nil
}

func (s *Store) IdentityUserByEmail(
	ctx context.Context,
	normalizedEmail string,
) (auth.User, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			id,
			email,
			email_normalized,
			display_name,
			password_hash,
			enabled,
			is_superadmin,
			created_at_unix,
			updated_at_unix
		FROM identity_users
		WHERE email_normalized = ?`,
		normalizedEmail,
	)

	return scanUser(row)
}

func (s *Store) CreateIdentitySession(
	ctx context.Context,
	input auth.SessionRecord,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO identity_sessions (
			id,
			user_id,
			token_hash,
			csrf_hash,
			expires_at_unix,
			created_at_unix,
			last_seen_at_unix,
			auth_method
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ID,
		input.UserID,
		input.TokenHash,
		input.CSRFHash,
		input.ExpiresAt.Unix(),
		input.CreatedAt.Unix(),
		input.LastSeenAt.Unix(),
		input.AuthMethod,
	)
	if err != nil {
		return fmt.Errorf("create identity session: %w", err)
	}

	return nil
}

func (s *Store) IdentitySessionByTokenHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (auth.Session, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT
			s.id,
			s.token_hash,
			s.csrf_hash,
			s.expires_at_unix,
			s.created_at_unix,
			s.last_seen_at_unix,
			s.auth_method,
			u.id,
			u.email,
			u.email_normalized,
			u.display_name,
			u.password_hash,
			u.enabled,
			u.is_superadmin,
			u.created_at_unix,
			u.updated_at_unix
		FROM identity_sessions s
		JOIN identity_users u ON u.id = s.user_id
		WHERE s.token_hash = ?
		  AND s.expires_at_unix > ?`,
		tokenHash,
		now.Unix(),
	)

	var (
		session     auth.Session
		expires     int64
		created     int64
		lastSeen    int64
		userCreated int64
		userUpdated int64
		enabled     int
		superadmin  int
	)

	err := row.Scan(
		&session.ID,
		&session.TokenHash,
		&session.CSRFHash,
		&expires,
		&created,
		&lastSeen,
		&session.AuthMethod,
		&session.User.ID,
		&session.User.Email,
		&session.User.EmailNormalized,
		&session.User.DisplayName,
		&session.User.PasswordHash,
		&enabled,
		&superadmin,
		&userCreated,
		&userUpdated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Session{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.Session{}, fmt.Errorf(
			"load identity session: %w",
			err,
		)
	}

	session.ExpiresAt = time.Unix(expires, 0).UTC()
	session.CreatedAt = time.Unix(created, 0).UTC()
	session.LastSeenAt = time.Unix(lastSeen, 0).UTC()
	session.User.Enabled = enabled == 1
	session.User.IsSuperadmin = superadmin == 1
	session.User.CreatedAt = time.Unix(userCreated, 0).UTC()
	session.User.UpdatedAt = time.Unix(userUpdated, 0).UTC()

	_, _ = s.db.ExecContext(
		ctx,
		`UPDATE identity_sessions
		 SET last_seen_at_unix = ?
		 WHERE token_hash = ?`,
		now.Unix(),
		tokenHash,
	)

	return session, nil
}

func (s *Store) UpdateIdentitySessionCSRF(
	ctx context.Context,
	tokenHash string,
	csrfHash string,
) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE identity_sessions
		 SET csrf_hash = ?
		 WHERE token_hash = ?`,
		csrfHash,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("update identity session csrf: %w", err)
	}

	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity session csrf rows affected: %w", err)
	}
	if changed == 0 {
		return auth.ErrNotFound
	}

	return nil
}

func (s *Store) DeleteIdentitySessionByTokenHash(
	ctx context.Context,
	tokenHash string,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM identity_sessions
		 WHERE token_hash = ?`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("delete identity session: %w", err)
	}

	return nil
}

func (s *Store) DeleteExpiredIdentitySessions(
	ctx context.Context,
	now time.Time,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM identity_sessions
		 WHERE expires_at_unix <= ?`,
		now.Unix(),
	)
	if err != nil {
		return fmt.Errorf("delete expired identity sessions: %w", err)
	}

	return nil
}

func (s *Store) RecordAuditEvent(
	ctx context.Context,
	event audit.Event,
) error {
	var userID any
	if event.UserID != nil {
		userID = *event.UserID
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO identity_audit_events (
			id,
			event_type,
			status,
			severity,
			user_id,
			remote_ip,
			reason_code,
			metadata_json,
			created_at_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		event.EventType,
		event.Status,
		event.Severity,
		userID,
		nullableString(event.RemoteIP),
		nullableString(event.ReasonCode),
		audit.MetadataJSON(event.Metadata),
		event.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("record identity audit event: %w", err)
	}

	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanUser(row rowScanner) (auth.User, error) {
	var (
		user       auth.User
		enabled    int
		superadmin int
		created    int64
		updated    int64
	)

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.EmailNormalized,
		&user.DisplayName,
		&user.PasswordHash,
		&enabled,
		&superadmin,
		&created,
		&updated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("scan identity user: %w", err)
	}

	user.Enabled = enabled == 1
	user.IsSuperadmin = superadmin == 1
	user.CreatedAt = time.Unix(created, 0).UTC()
	user.UpdatedAt = time.Unix(updated, 0).UTC()

	return user, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
