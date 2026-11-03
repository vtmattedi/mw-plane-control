package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
)

func (s *Store) CreateApplication(
	ctx context.Context,
	input applications.CreateInput,
) error {
	if input.AccessPolicy == "" {
		input.AccessPolicy = "default_deny"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf(
			"begin create application: %w",
			err,
		)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var secret any
	if input.ClientSecretHash != "" {
		secret = input.ClientSecretHash
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO identity_applications (
			id,
			name,
			client_id,
			client_secret_hash,
			application_type,
			authentication_policy,
			access_policy,
			enabled,
			archived_at_unix,
			created_at_unix,
			updated_at_unix,
			delete_callback_uri,
			allow_public_signup,
			internal, access_policy_v1
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?)`,
		input.ID,
		input.Name,
		input.ClientID,
		secret,
		input.ApplicationType,
		input.AuthenticationPolicy,
		input.AccessPolicy,
		boolInt(input.Enabled),
		input.CreatedAt.Unix(),
		input.CreatedAt.Unix(),
		nullableString(input.DeleteCallbackURI),
		boolInt(input.AllowPublicSignup),
		boolInt(input.Internal), boolInt(input.AccessPolicyV1),
	)
	if err != nil {
		if strings.Contains(
			strings.ToLower(err.Error()),
			"unique",
		) {
			return applications.ErrClientIDExists
		}
		return fmt.Errorf(
			"insert identity application: %w",
			err,
		)
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_redirect_uris",
		"redirect_uri",
		input.ID,
		input.RedirectURIs,
		input.CreatedAt,
	); err != nil {
		return err
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_post_logout_redirect_uris",
		"redirect_uri",
		input.ID,
		input.PostLogoutRedirectURIs,
		input.CreatedAt,
	); err != nil {
		return err
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_scopes",
		"scope",
		input.ID,
		input.Scopes,
		input.CreatedAt,
	); err != nil {
		return err
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_claims",
		"claim",
		input.ID,
		input.Claims,
		input.CreatedAt,
	); err != nil {
		return err
	}

	if err := insertValues(
		ctx,
		tx,
		"identity_application_audiences",
		"audience",
		input.ID,
		input.Audiences,
		input.CreatedAt,
	); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"commit create application: %w",
			err,
		)
	}

	return nil
}

func (s *Store) ApplicationByClientID(
	ctx context.Context,
	clientID string,
) (applications.Application, error) {
	var (
		app               applications.Application
		secret            sql.NullString
		archived          sql.NullInt64
		enabled           int
		created           int64
		updated           int64
		deleteCallback    sql.NullString
		allowPublicSignup int
		internal          int
	)

	err := s.db.QueryRowContext(
		ctx,
		`SELECT
			id,
			name,
			client_id,
			client_secret_hash,
			application_type,
			authentication_policy,
			access_policy,
			enabled,
			archived_at_unix,
			created_at_unix,
			updated_at_unix,
			delete_callback_uri,
			allow_public_signup,
			internal, access_policy_v1
		FROM identity_applications
		WHERE client_id = ?`,
		clientID,
	).Scan(
		&app.ID,
		&app.Name,
		&app.ClientID,
		&secret,
		&app.ApplicationType,
		&app.AuthenticationPolicy,
		&app.AccessPolicy,
		&enabled,
		&archived,
		&created,
		&updated,
		&deleteCallback,
		&allowPublicSignup,
		&internal, &app.AccessPolicyV1,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return applications.Application{},
			applications.ErrApplicationNotFound
	}
	if err != nil {
		return applications.Application{},
			fmt.Errorf(
				"load identity application: %w",
				err,
			)
	}

	app.ClientSecretHash = secret.String
	app.DeleteCallbackURI = deleteCallback.String
	app.AllowPublicSignup = allowPublicSignup == 1
	app.Internal = internal == 1
	app.NeedsDeleteCallback = !app.Internal && app.DeleteCallbackURI == ""
	app.Enabled = enabled == 1
	app.CreatedAt = time.Unix(created, 0).UTC()
	app.UpdatedAt = time.Unix(updated, 0).UTC()

	if archived.Valid {
		value := time.Unix(
			archived.Int64,
			0,
		).UTC()
		app.ArchivedAt = &value
	}

	app.RedirectURIs, err = loadValues(
		ctx,
		s.db,
		"identity_application_redirect_uris",
		"redirect_uri",
		app.ID,
	)
	if err != nil {
		return applications.Application{}, err
	}

	app.PostLogoutRedirectURIs, err = loadValues(
		ctx,
		s.db,
		"identity_application_post_logout_redirect_uris",
		"redirect_uri",
		app.ID,
	)
	if err != nil {
		return applications.Application{}, err
	}

	app.Scopes, err = loadValues(
		ctx,
		s.db,
		"identity_application_scopes",
		"scope",
		app.ID,
	)
	if err != nil {
		return applications.Application{}, err
	}

	app.Claims, err = loadValues(
		ctx,
		s.db,
		"identity_application_claims",
		"claim",
		app.ID,
	)
	if err != nil {
		return applications.Application{}, err
	}

	app.Audiences, err = loadValues(
		ctx,
		s.db,
		"identity_application_audiences",
		"audience",
		app.ID,
	)
	if err != nil {
		return applications.Application{}, err
	}

	return app, nil
}

func insertValues(
	ctx context.Context,
	tx *sql.Tx,
	table string,
	column string,
	applicationID string,
	values []string,
	createdAt time.Time,
) error {
	query := fmt.Sprintf(
		`INSERT INTO %s (
			application_id,
			%s,
			created_at_unix
		) VALUES (?, ?, ?)`,
		table,
		column,
	)

	for _, value := range values {
		if _, err := tx.ExecContext(
			ctx,
			query,
			applicationID,
			value,
			createdAt.Unix(),
		); err != nil {
			return fmt.Errorf(
				"insert %s value: %w",
				table,
				err,
			)
		}
	}

	return nil
}

func loadValues(
	ctx context.Context,
	db *sql.DB,
	table string,
	column string,
	applicationID string,
) ([]string, error) {
	query := fmt.Sprintf(
		`SELECT %s
		 FROM %s
		 WHERE application_id = ?
		 ORDER BY %s ASC`,
		column,
		table,
		column,
	)

	rows, err := db.QueryContext(
		ctx,
		query,
		applicationID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"load %s values: %w",
			table,
			err,
		)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf(
				"scan %s value: %w",
				table,
				err,
			)
		}
		result = append(result, value)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate %s values: %w",
			table,
			err,
		)
	}

	return result, nil
}
