package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

// CutoverFederationAccess requires an operator's explicit reviewed policy.
// It never guesses the intention of a legacy signup-enabled private client.
func (s *Store) CutoverFederationAccess(ctx context.Context, clientID, policy string) error {
	if policy != "public" && policy != "private" {
		return fmt.Errorf("explicit public/private policy required")
	}
	stored := "default_deny"
	if policy == "public" {
		stored = "default_allow"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE identity_applications SET access_policy=?,access_policy_v1=1,updated_at_unix=? WHERE client_id=? AND archived_at_unix IS NULL`, stored, time.Now().Unix(), clientID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return auth.ErrNotFound
	}
	for _, table := range []string{"identity_authorization_codes", "identity_access_tokens"} {
		if _, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE client_id=?`, clientID); err != nil {
			return err
		}
	}
	id, err := auth.NewOpaqueToken("aud_")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_audit_events(id,event_type,status,severity,reason_code,metadata_json,created_at_unix) VALUES (?,'identity.application.access_v1','success','warning','operator_review','{}',?)`, id, time.Now().Unix())
	if err != nil {
		return err
	}
	return tx.Commit()
}

// RetireApplicationStrong is host-only and requires recorded consumer proof review.
func (s *Store) RetireApplicationStrong(ctx context.Context, client string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `UPDATE identity_applications SET authentication_policy='standard' WHERE client_id=? AND archived_at_unix IS NULL`, client)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return auth.ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM identity_authorization_codes WHERE client_id=?`, client); err != nil {
		return err
	}
	id, err := auth.NewOpaqueToken("aud_")
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO identity_audit_events(id,event_type,status,severity,reason_code,metadata_json,created_at_unix) VALUES (?,'identity.application.action_assurance_cutover','success','warning','consumer_review','{}',?)`, id, time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}
