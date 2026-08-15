package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identityadmin"
)

func (s *Store) ListIdentityApplications(
	ctx context.Context,
) ([]applications.Application, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id
		 FROM identity_applications
		 ORDER BY created_at_unix DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list Identity applications: %w",
			err,
		)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make(
		[]applications.Application,
		0,
		len(ids),
	)
	for _, id := range ids {
		app, err := s.IdentityApplicationByID(
			ctx,
			id,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, app)
	}

	return result, nil
}

func (s *Store) IdentityApplicationByID(
	ctx context.Context,
	id string,
) (applications.Application, error) {
	var clientID string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT client_id
		 FROM identity_applications
		 WHERE id = ?`,
		id,
	).Scan(&clientID)

	if errors.Is(err, sql.ErrNoRows) {
		return applications.Application{},
			identityadmin.ErrNotFound
	}
	if err != nil {
		return applications.Application{},
			fmt.Errorf(
				"load Identity application id: %w",
				err,
			)
	}

	app, err := s.ApplicationByClientID(
		ctx,
		clientID,
	)
	if errors.Is(
		err,
		applications.ErrApplicationNotFound,
	) {
		return applications.Application{},
			identityadmin.ErrNotFound
	}
	return app, err
}

func (s *Store) UpdateIdentityApplication(
	ctx context.Context,
	id string,
	value identityadmin.UpdateApplication,
	now time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf(
			"begin Identity application update: %w",
			err,
		)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := tx.ExecContext(
		ctx,
		`UPDATE identity_applications
		 SET name = ?,
		     authentication_policy = ?,
		     delete_callback_uri = ?,
		     allow_public_signup = ?,
		     updated_at_unix = ?
		 WHERE id = ?
		   AND archived_at_unix IS NULL`,
		value.Name,
		value.AuthenticationPolicy,
		nullableString(value.DeleteCallbackURI),
		boolInt(value.AllowPublicSignup),
		now.Unix(),
		id,
	)
	if err != nil {
		return fmt.Errorf(
			"update Identity application: %w",
			err,
		)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return identityadmin.ErrNotFound
	}

	for _, table := range []string{
		"identity_application_redirect_uris",
		"identity_application_post_logout_redirect_uris",
		"identity_application_scopes",
		"identity_application_claims",
		"identity_application_audiences",
	} {
		if _, err := tx.ExecContext(
			ctx,
			fmt.Sprintf(
				"DELETE FROM %s WHERE application_id = ?",
				table,
			),
			id,
		); err != nil {
			return fmt.Errorf(
				"clear %s: %w",
				table,
				err,
			)
		}
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_redirect_uris",
		"redirect_uri",
		id,
		value.RedirectURIs,
		now,
	); err != nil {
		return err
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_post_logout_redirect_uris",
		"redirect_uri",
		id,
		value.PostLogoutRedirectURIs,
		now,
	); err != nil {
		return err
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_scopes",
		"scope",
		id,
		value.Scopes,
		now,
	); err != nil {
		return err
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_claims",
		"claim",
		id,
		value.Claims,
		now,
	); err != nil {
		return err
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_audiences",
		"audience",
		id,
		value.Audiences,
		now,
	); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"commit Identity application update: %w",
			err,
		)
	}

	return nil
}

func (s *Store) SetIdentityApplicationState(
	ctx context.Context,
	id string,
	enabled bool,
	archivedAt *time.Time,
	now time.Time,
) error {
	var archived any
	if archivedAt != nil {
		archived = archivedAt.Unix()
	}

	result, err := s.db.ExecContext(
		ctx,
		`UPDATE identity_applications
		 SET enabled = ?,
		     archived_at_unix = ?,
		     updated_at_unix = ?
		 WHERE id = ?`,
		boolInt(enabled),
		archived,
		now.Unix(),
		id,
	)
	if err != nil {
		return fmt.Errorf(
			"set Identity application state: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return identityadmin.ErrNotFound
	}
	return nil
}

func (s *Store) PurgeIdentityApplication(
	ctx context.Context,
	id string,
	clientID string,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Identity application purge: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var dependencies int
	err = tx.QueryRowContext(
		ctx,
		`SELECT
			(SELECT COUNT(1) FROM identity_application_assignments WHERE application_id = ?) +
			(SELECT COUNT(1) FROM identity_authorization_codes WHERE client_id = ?) +
			(SELECT COUNT(1) FROM identity_access_tokens WHERE client_id = ?)`,
		id,
		clientID,
		clientID,
	).Scan(&dependencies)
	if err != nil {
		return fmt.Errorf("check Identity application dependencies: %w", err)
	}
	if dependencies != 0 {
		return identityadmin.ErrApplicationInUse
	}

	result, err := tx.ExecContext(
		ctx,
		`DELETE FROM identity_applications
		 WHERE id = ?
		   AND client_id = ?
		   AND archived_at_unix IS NOT NULL`,
		id,
		clientID,
	)
	if err != nil {
		return fmt.Errorf("purge Identity application: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return identityadmin.ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Identity application purge: %w", err)
	}
	return nil
}

func (s *Store) UpdateIdentityApplicationSecretHash(
	ctx context.Context,
	id string,
	secretHash string,
	now time.Time,
) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE identity_applications
		 SET client_secret_hash = ?,
		     updated_at_unix = ?
		 WHERE id = ?
		   AND application_type = 'confidential'
		   AND archived_at_unix IS NULL`,
		secretHash,
		now.Unix(),
		id,
	)
	if err != nil {
		return fmt.Errorf(
			"rotate Identity application secret: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return identityadmin.ErrNotFound
	}
	return nil
}

func (s *Store) ListIdentityUsers(
	ctx context.Context,
) ([]identityadmin.UserSummary, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
			id,
			email,
			display_name,
			enabled,
			is_superadmin,
			account_type,
			username,
			email_verified_at_unix,
			deletion_requested_at_unix,
			created_at_unix
		 FROM identity_users
		 ORDER BY created_at_unix DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list Identity users: %w",
			err,
		)
	}
	defer rows.Close()

	var result []identityadmin.UserSummary
	for rows.Next() {
		var (
			user              identityadmin.UserSummary
			enabled           int
			superadmin        int
			createdUnix       int64
			emailVerified     sql.NullInt64
			deletionRequested sql.NullInt64
		)

		if err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.DisplayName,
			&enabled,
			&superadmin,
			&user.AccountType,
			&user.Username,
			&emailVerified,
			&deletionRequested,
			&createdUnix,
		); err != nil {
			return nil, err
		}

		user.Enabled = enabled == 1
		user.IsSuperadmin = superadmin == 1
		user.CreatedAt = time.Unix(
			createdUnix,
			0,
		).UTC()
		if emailVerified.Valid {
			value := time.Unix(emailVerified.Int64, 0).UTC()
			user.EmailVerifiedAt = &value
		}
		if deletionRequested.Valid {
			value := time.Unix(deletionRequested.Int64, 0).UTC()
			user.DeletionRequestedAt = &value
		}

		result = append(result, user)
	}

	return result, rows.Err()
}

func (s *Store) ListIdentityAssignments(
	ctx context.Context,
	applicationID string,
) ([]identityadmin.Assignment, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
			application_id,
			user_id,
			role,
			created_at_unix,
			updated_at_unix
		 FROM identity_application_assignments
		 WHERE application_id = ?
		 ORDER BY created_at_unix ASC, user_id ASC`,
		applicationID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list Identity assignments: %w",
			err,
		)
	}
	defer rows.Close()

	var result []identityadmin.Assignment
	for rows.Next() {
		var (
			value       identityadmin.Assignment
			createdUnix int64
			updatedUnix int64
		)
		if err := rows.Scan(
			&value.ApplicationID,
			&value.UserID,
			&value.Role,
			&createdUnix,
			&updatedUnix,
		); err != nil {
			return nil, err
		}

		value.CreatedAt = time.Unix(
			createdUnix,
			0,
		).UTC()
		value.UpdatedAt = time.Unix(
			updatedUnix,
			0,
		).UTC()

		result = append(result, value)
	}

	return result, rows.Err()
}

func (s *Store) UpsertIdentityAssignment(
	ctx context.Context,
	value identityadmin.Assignment,
) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO identity_application_assignments (
			application_id,
			user_id,
			role,
			created_at_unix,
			updated_at_unix
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(application_id, user_id)
		DO UPDATE SET
			role = excluded.role,
			updated_at_unix = excluded.updated_at_unix`,
		value.ApplicationID,
		value.UserID,
		value.Role,
		value.CreatedAt.Unix(),
		value.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf(
			"upsert Identity assignment: %w",
			err,
		)
	}
	return nil
}

func (s *Store) DeleteIdentityAssignment(
	ctx context.Context,
	applicationID string,
	userID string,
) error {
	result, err := s.db.ExecContext(
		ctx,
		`DELETE FROM identity_application_assignments
		 WHERE application_id = ?
		   AND user_id = ?`,
		applicationID,
		userID,
	)
	if err != nil {
		return fmt.Errorf(
			"delete Identity assignment: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return identityadmin.ErrNotFound
	}
	return nil
}

func (s *Store) IdentityAssignmentRole(
	ctx context.Context,
	applicationID string,
	userID string,
) (string, error) {
	var role string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT role
		 FROM identity_application_assignments
		 WHERE application_id = ?
		   AND user_id = ?`,
		applicationID,
		userID,
	).Scan(&role)

	if errors.Is(err, sql.ErrNoRows) {
		return "", identityadmin.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf(
			"load Identity assignment: %w",
			err,
		)
	}
	return role, nil
}

func (s *Store) IdentityAssignmentRoleByClientID(
	ctx context.Context,
	clientID string,
	userID string,
) (string, error) {
	var role string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT a.role
		 FROM identity_application_assignments a
		 JOIN identity_applications p
		   ON p.id = a.application_id
		 WHERE p.client_id = ?
		   AND a.user_id = ?`,
		clientID,
		userID,
	).Scan(&role)

	if errors.Is(err, sql.ErrNoRows) {
		return "", identityadmin.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf(
			"load Identity assignment by client: %w",
			err,
		)
	}
	return role, nil
}

func (s *Store) ListIdentityAudit(
	ctx context.Context,
	limit int,
) ([]identityadmin.AuditEntry, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT
			id,
			event_type,
			status,
			severity,
			user_id,
			COALESCE(remote_ip, ''),
			COALESCE(reason_code, ''),
			metadata_json,
			created_at_unix
		 FROM identity_audit_events
		 ORDER BY created_at_unix DESC, id DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list Identity audit: %w",
			err,
		)
	}
	defer rows.Close()

	var result []identityadmin.AuditEntry
	for rows.Next() {
		var (
			value       identityadmin.AuditEntry
			userID      sql.NullString
			metadata    string
			createdUnix int64
		)

		if err := rows.Scan(
			&value.ID,
			&value.EventType,
			&value.Status,
			&value.Severity,
			&userID,
			&value.RemoteIP,
			&value.ReasonCode,
			&metadata,
			&createdUnix,
		); err != nil {
			return nil, err
		}

		if userID.Valid {
			id := userID.String
			value.UserID = &id
		}

		if !json.Valid([]byte(metadata)) {
			metadata = "{}"
		}
		value.MetadataJSON = json.RawMessage(
			metadata,
		)
		value.CreatedAt = time.Unix(
			createdUnix,
			0,
		).UTC()

		result = append(result, value)
	}

	return result, rows.Err()
}
