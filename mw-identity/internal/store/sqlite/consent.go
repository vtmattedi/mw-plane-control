package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/oidc"
)

func (s *Store) IdentityConsent(ctx context.Context, user, app string) (oidc.Consent, error) {
	var c oidc.Consent
	var scopes string
	err := s.db.QueryRowContext(ctx, `SELECT version,active,scopes_json FROM identity_application_consents WHERE user_id=? AND application_id=?`, user, app).Scan(&c.Version, &c.Active, &scopes)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal([]byte(scopes), &c.Scopes)
	return c, err
}

func (s *Store) GrantIdentityConsent(ctx context.Context, user, app string, scopes []string, expected int64, now time.Time) error {
	body, err := json.Marshal(scopes)
	if err != nil {
		return err
	}
	var result sql.Result
	if expected == 0 {
		result, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO identity_application_consents
 (user_id,application_id,active,version,scopes_json,created_at_unix,updated_at_unix)
 VALUES (?,?,1,1,?,?,?)`, user, app, string(body), now.Unix(), now.Unix())
	} else {
		result, err = s.db.ExecContext(ctx, `UPDATE identity_application_consents SET active=1,version=version+1,scopes_json=?,updated_at_unix=? WHERE user_id=? AND application_id=? AND version=?`, string(body), now.Unix(), user, app, expected)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return oidc.ErrStoreNotFound
	}
	return nil
}

// Shared by INSERT ... SELECT and reads: each SQL statement is atomic with
// disconnect's transaction, including an exchange already holding an old code.
const credentialConsentPredicate = `EXISTS (SELECT 1 FROM identity_applications p
 JOIN identity_users u ON u.id=?
 WHERE p.client_id=? AND p.enabled=1 AND p.archived_at_unix IS NULL AND u.enabled=1
 AND ` + credentialEligibility + `
 AND (? > 0 AND EXISTS (SELECT 1 FROM identity_application_consents c
 WHERE c.user_id=u.id AND c.application_id=p.id AND c.active=1 AND c.version=?)))`

// Mirrors AuthorizeIdentityUser; exercised against the same policy matrix.
const credentialEligibility = `((u.is_superadmin=1 AND (p.access_policy_v1=0 OR EXISTS (SELECT 1 FROM identity_application_audiences aa WHERE aa.application_id=p.id AND aa.audience='mw-identity-admin'))) OR EXISTS (SELECT 1 FROM identity_application_assignments a WHERE a.user_id=u.id AND a.application_id=p.id AND a.access_effect='allow') OR (NOT EXISTS (SELECT 1 FROM identity_application_assignments a WHERE a.user_id=u.id AND a.application_id=p.id) AND (p.access_policy='default_allow' OR (p.access_policy_v1=0 AND p.allow_public_signup=1 AND p.internal=0 AND NOT EXISTS (SELECT 1 FROM identity_application_audiences aa WHERE aa.application_id=p.id AND aa.audience='mw-identity-admin'))))) AND (u.account_type!='public' OR NOT EXISTS (SELECT 1 FROM identity_application_audiences aa WHERE aa.application_id=p.id AND aa.audience='mw-identity-admin')) AND u.deletion_requested_at_unix IS NULL`
