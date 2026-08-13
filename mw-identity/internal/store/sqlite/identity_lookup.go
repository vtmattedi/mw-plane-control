package sqlite

import (
	"context"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

func (s *Store) IdentityUserByID(
	ctx context.Context,
	userID string,
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
		WHERE id = ?`,
		userID,
	)

	return scanUser(row)
}
