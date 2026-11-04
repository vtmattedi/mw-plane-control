package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"time"
)

func (s *Store) BeginDurableDeletion(ctx context.Context, subject string, now time.Time, capability string, expiry time.Time) (auth.DeletionOperation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.DeletionOperation{}, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM identity_deletion_operations WHERE subject=?`, subject).Scan(&id)
	if err == sql.ErrNoRows {
		id, err = auth.NewOpaqueToken("del_")
		if err != nil {
			return auth.DeletionOperation{}, err
		}
		result, err := tx.ExecContext(ctx, `UPDATE identity_users SET enabled=0,deletion_requested_at_unix=?,updated_at_unix=? WHERE id=? AND is_superadmin=0 AND deletion_requested_at_unix IS NULL`, now.Unix(), now.Unix(), subject)
		if err != nil {
			return auth.DeletionOperation{}, err
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return auth.DeletionOperation{}, auth.ErrAccountDeletionForbidden
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity_deletion_operations(id,subject,requested_at_unix,status) VALUES (?,?,?,'pending')`, id, subject, now.Unix()); err != nil {
			return auth.DeletionOperation{}, err
		}
		rows, err := tx.QueryContext(ctx, `WITH known AS (SELECT application_id FROM identity_application_consents WHERE user_id=? UNION SELECT application_id FROM identity_application_assignments WHERE user_id=?) SELECT p.id,p.client_id,p.name,COALESCE(p.delete_callback_uri,''),p.internal FROM identity_applications p JOIN known k ON k.application_id=p.id`, subject, subject)
		if err != nil {
			return auth.DeletionOperation{}, err
		}
		type target struct {
			id, client, name, callback string
			internal                   bool
		}
		var targets []target
		for rows.Next() {
			var t target
			if err = rows.Scan(&t.id, &t.client, &t.name, &t.callback, &t.internal); err != nil {
				rows.Close()
				return auth.DeletionOperation{}, err
			}
			targets = append(targets, t)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return auth.DeletionOperation{}, err
		}
		for _, t := range targets {
			event, err := auth.NewOpaqueToken("evt_")
			if err != nil {
				return auth.DeletionOperation{}, err
			}
			status := auth.InitialDeletionStatus(t.internal, t.callback)
			if _, err = tx.ExecContext(ctx, `INSERT INTO identity_deletion_deliveries(event_id,operation_id,application_id,client_id,application_name,callback_uri,status,next_attempt_at_unix,retry_started_at_unix) VALUES (?,?,?,?,?,?,?,?,?)`, event, id, t.id, t.client, t.name, t.callback, status, now.Unix(), now.Unix()); err != nil {
				return auth.DeletionOperation{}, err
			}
		}
	} else if err != nil {
		return auth.DeletionOperation{}, err
	}
	// Operator retries must not replace the initiating browser's credential or
	// extend its lifetime. Only the first request can issue a status capability.
	credential, err := tx.ExecContext(ctx, `UPDATE identity_deletion_operations SET capability_hash=?,capability_expires_at_unix=? WHERE id=? AND capability_hash IS NULL`, capability, expiry.Unix(), id)
	if err != nil {
		return auth.DeletionOperation{}, err
	}
	issued, err := credential.RowsAffected()
	if err != nil {
		return auth.DeletionOperation{}, err
	}
	for _, table := range []string{"identity_sessions", "identity_authorization_codes", "identity_access_tokens", "identity_mfa_challenges"} {
		if _, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE user_id=?`, subject); err != nil {
			return auth.DeletionOperation{}, err
		}
	}
	auditID, auditErr := auth.NewOpaqueToken("aud_")
	if auditErr != nil {
		return auth.DeletionOperation{}, auditErr
	}
	metadata, _ := json.Marshal(map[string]string{"operation_id": id})
	if _, err = tx.ExecContext(ctx, `INSERT INTO identity_audit_events(id,event_type,status,severity,reason_code,metadata_json,created_at_unix) VALUES (?,'identity.deletion.requested','success','warning','account_deleted',?,?)`, auditID, string(metadata), now.Unix()); err != nil {
		return auth.DeletionOperation{}, err
	}
	if err = tx.Commit(); err != nil {
		return auth.DeletionOperation{}, err
	}
	if err = s.FinalizeDurableDeletion(ctx, id, now); err != nil {
		return auth.DeletionOperation{}, err
	}
	op, err := s.DeletionOperation(ctx, id)
	op.StatusCredentialIssued = issued == 1
	return op, err
}
func (s *Store) DeletionOperation(ctx context.Context, id string) (auth.DeletionOperation, error) {
	var op auth.DeletionOperation
	var requested int64
	err := s.db.QueryRowContext(ctx, `SELECT id,subject,requested_at_unix,status FROM identity_deletion_operations WHERE id=?`, id).Scan(&op.ID, &op.Subject, &requested, &op.Status)
	if err != nil {
		return op, err
	}
	op.RequestedAt = time.Unix(requested, 0).UTC()
	rows, err := s.db.QueryContext(ctx, `SELECT event_id,application_id,client_id,application_name,callback_uri,status,attempts,next_attempt_at_unix,error_code FROM identity_deletion_deliveries WHERE operation_id=? ORDER BY application_name,event_id`, id)
	if err != nil {
		return op, err
	}
	defer rows.Close()
	op.Deliveries = []auth.DurableDelivery{}
	for rows.Next() {
		var d auth.DurableDelivery
		var next int64
		if err = rows.Scan(&d.EventID, &d.ApplicationID, &d.ClientID, &d.Name, &d.CallbackURI, &d.Status, &d.Attempts, &next, &d.ErrorCode); err != nil {
			return op, err
		}
		d.OperationID = id
		d.NextAttempt = time.Unix(next, 0).UTC()
		op.Deliveries = append(op.Deliveries, d)
	}
	return op, rows.Err()
}
func (s *Store) DeletionByCapability(ctx context.Context, hash string, now time.Time) (auth.DeletionOperation, error) {
	var id string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM identity_deletion_operations WHERE capability_hash=? AND capability_expires_at_unix>?`, hash, now.Unix()).Scan(&id); err != nil {
		return auth.DeletionOperation{}, auth.ErrNotFound
	}
	return s.DeletionOperation(ctx, id)
}
func (s *Store) ClaimDeletionDelivery(ctx context.Context, now time.Time, lease string) (auth.DurableDelivery, error) {
	var d auth.DurableDelivery
	var retryStarted int64
	err := s.db.QueryRowContext(ctx, `UPDATE identity_deletion_deliveries SET status='delivering',attempts=attempts+1,lease_token=?,lease_until_unix=? WHERE event_id=(SELECT d.event_id FROM identity_deletion_deliveries d JOIN identity_deletion_operations o ON o.id=d.operation_id WHERE o.status='pending' AND ((d.status='pending' AND d.next_attempt_at_unix<=?) OR (d.status='delivering' AND d.lease_until_unix<=?)) ORDER BY d.next_attempt_at_unix,d.event_id LIMIT 1) RETURNING event_id,operation_id,application_id,client_id,application_name,callback_uri,attempts,retry_started_at_unix`, lease, now.Add(30*time.Second).Unix(), now.Unix(), now.Unix()).Scan(&d.EventID, &d.OperationID, &d.ApplicationID, &d.ClientID, &d.Name, &d.CallbackURI, &d.Attempts, &retryStarted)
	if err == sql.ErrNoRows {
		return d, auth.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.Lease = lease
	d.RetryStartedAt = time.Unix(retryStarted, 0).UTC()
	var requested int64
	if err = s.db.QueryRowContext(ctx, `SELECT subject,requested_at_unix FROM identity_deletion_operations WHERE id=?`, d.OperationID).Scan(&d.Subject, &requested); err != nil {
		return d, err
	}
	d.RequestedAt = time.Unix(requested, 0).UTC()
	// Registry changes require an explicit operator snapshot refresh, never silent redirection.
	var current string
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(delete_callback_uri,'') FROM identity_applications WHERE id=? AND client_id=?`, d.ApplicationID, d.ClientID).Scan(&current)
	if err == sql.ErrNoRows || (err == nil && current != d.CallbackURI) {
		d.ErrorCode = "callback_changed"
	} else if err != nil {
		return d, err
	}
	return d, nil
}
func (s *Store) FinishDeletionDelivery(ctx context.Context, d auth.DurableDelivery, status, code string, next time.Time) error {
	r, err := s.db.ExecContext(ctx, `UPDATE identity_deletion_deliveries SET status=?,error_code=?,next_attempt_at_unix=?,lease_token='',lease_until_unix=0 WHERE event_id=? AND status='delivering' AND lease_token=?`, status, code, next.Unix(), d.EventID, d.Lease)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return auth.ErrNotFound
	}
	return nil
}
func (s *Store) FinalizeDurableDeletion(ctx context.Context, id string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var subject string
	var remaining int
	if err = tx.QueryRowContext(ctx, `SELECT subject FROM identity_deletion_operations WHERE id=? AND status='pending'`, id).Scan(&subject); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_deletion_deliveries WHERE operation_id=? AND status NOT IN ('confirmed_cleanup','no_current_mapping','exempt')`, id).Scan(&remaining); err != nil {
		return err
	}
	if remaining > 0 {
		return nil
	}
	var eligible int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_users WHERE id=? AND enabled=0 AND deletion_requested_at_unix IS NOT NULL`, subject).Scan(&eligible); err != nil {
		return err
	}
	if eligible != 1 {
		return fmt.Errorf("deletion subject state inconsistent")
	}
	deleted, err := tx.ExecContext(ctx, `DELETE FROM identity_users WHERE id=? AND enabled=0 AND deletion_requested_at_unix IS NOT NULL`, subject)
	if err != nil {
		return err
	}
	count, err := deleted.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("deletion subject was not removed")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE identity_deletion_operations SET status='completed',completed_at_unix=? WHERE id=?`, now.Unix(), id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) RetryDeletionDelivery(ctx context.Context, event string, refresh bool, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if refresh {
		_, err = tx.ExecContext(ctx, `UPDATE identity_deletion_deliveries SET callback_uri=COALESCE((SELECT p.delete_callback_uri FROM identity_applications p WHERE p.id=identity_deletion_deliveries.application_id AND p.client_id=identity_deletion_deliveries.client_id),callback_uri) WHERE event_id=? AND status IN ('failed','unresolved')`, event)
		if err != nil {
			return err
		}
	}
	r, err := tx.ExecContext(ctx, `UPDATE identity_deletion_deliveries SET status='pending',next_attempt_at_unix=?,retry_started_at_unix=?,error_code='',lease_token='',lease_until_unix=0 WHERE event_id=? AND callback_uri!='' AND status IN ('failed','unresolved','pending') AND operation_id IN (SELECT id FROM identity_deletion_operations WHERE status='pending')`, now.Unix(), now.Unix(), event)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return auth.ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) FinalizeReadyDeletions(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM identity_deletion_operations o WHERE status='pending' AND NOT EXISTS (SELECT 1 FROM identity_deletion_deliveries d WHERE d.operation_id=o.id AND d.status NOT IN ('confirmed_cleanup','no_current_mapping','exempt'))`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = s.FinalizeDurableDeletion(ctx, id, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListDeletionOperations(ctx context.Context) ([]auth.DeletionOperation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,subject,requested_at_unix,status FROM identity_deletion_operations ORDER BY requested_at_unix DESC,id LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ops := []auth.DeletionOperation{}
	for rows.Next() {
		var op auth.DeletionOperation
		var at int64
		if err = rows.Scan(&op.ID, &op.Subject, &at, &op.Status); err != nil {
			return nil, err
		}
		op.RequestedAt = time.Unix(at, 0).UTC()
		ops = append(ops, op)
	}
	return ops, rows.Err()
}
func (s *Store) DeletionQueueStats(ctx context.Context, now time.Time) (map[string]int64, error) {
	var pending, failed, oldest, operations int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN d.status IN ('failed','unresolved') THEN 1 ELSE 0 END),0),COALESCE(MIN(o.requested_at_unix),?), (SELECT COUNT(*) FROM identity_deletion_operations WHERE status!='completed') FROM identity_deletion_deliveries d JOIN identity_deletion_operations o ON o.id=d.operation_id WHERE d.status IN ('pending','delivering','failed','unresolved')`, now.Unix()).Scan(&pending, &failed, &oldest, &operations)
	return map[string]int64{"outstanding": pending, "failed_or_unresolved": failed, "oldest_age_seconds": max(int64(0), now.Unix()-oldest), "pending_operations": operations}, err
}
func (s *Store) ReconcileLegacyDeletion(ctx context.Context, id string, outcomes map[string]string, reference string, now time.Time) error {
	if reference == "" || len(reference) > 500 {
		return fmt.Errorf("review evidence reference required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var subject string
	if err = tx.QueryRowContext(ctx, `SELECT subject FROM identity_deletion_operations WHERE id=? AND status='legacy_review_required'`, id).Scan(&subject); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `WITH known AS (SELECT application_id FROM identity_application_consents WHERE user_id=? UNION SELECT application_id FROM identity_application_assignments WHERE user_id=?) SELECT p.id,p.client_id,p.name,COALESCE(p.delete_callback_uri,''),p.internal FROM identity_applications p JOIN known k ON k.application_id=p.id`, subject, subject)
	if err != nil {
		return err
	}
	type receipt struct{ id, client, name, uri, status string }
	var receipts []receipt
	for rows.Next() {
		var r receipt
		var internal bool
		if err = rows.Scan(&r.id, &r.client, &r.name, &r.uri, &internal); err != nil {
			rows.Close()
			return err
		}
		r.status = outcomes[r.client]
		if internal {
			r.status = "exempt"
		} else if r.status != "confirmed_cleanup" && r.status != "no_current_mapping" {
			rows.Close()
			return fmt.Errorf("every ordinary candidate needs verified outcome")
		}
		receipts = append(receipts, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range receipts {
		receiptID, err := auth.NewOpaqueToken("receipt_")
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO identity_deletion_deliveries(event_id,operation_id,application_id,client_id,application_name,callback_uri,status,next_attempt_at_unix,retry_started_at_unix) VALUES (?,?,?,?,?,?,?,?,?)`, receiptID, id, r.id, r.client, r.name, r.uri, r.status, now.Unix(), now.Unix()); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE identity_deletion_operations SET status='pending' WHERE id=?`, id); err != nil {
		return err
	}
	metadata, _ := json.Marshal(map[string]any{"operation_id": id, "evidence_reference": reference})
	auditID, err := auth.NewOpaqueToken("aud_")
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO identity_audit_events(id,event_type,status,severity,reason_code,metadata_json,created_at_unix) VALUES (?,'identity.deletion.legacy_reconciled','success','warning','operator_review',?,?)`, auditID, string(metadata), now.Unix()); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return s.FinalizeDurableDeletion(ctx, id, now)
}
