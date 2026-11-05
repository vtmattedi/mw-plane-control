package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

var (
	ErrAdminBFFRepairNotFound   = errors.New("admin BFF client not found")
	ErrAdminBFFRepairIneligible = errors.New("client is not eligible for admin BFF repair")
)

// AdminBFFRepairResult describes the only state this operator command can change.
type AdminBFFRepairResult struct {
	ApplicationID   string
	ClientID        string
	Name            string
	AlreadyInternal bool
	Changed         bool
}

// RepairAdminBFFInternal promotes exactly one verified confidential admin BFF
// registration. It is intentionally NOT exposed through HTTP or the normal
// application administration service. A dry run is read-only. The update and
// audit record share a transaction, so an audit failure rolls back the change.
func (s *Store) RepairAdminBFFInternal(
	ctx context.Context,
	clientID string,
	expectedRedirectURI string,
	apply bool,
	now time.Time,
) (AdminBFFRepairResult, error) {
	clientID = strings.TrimSpace(clientID)
	expectedRedirectURI = strings.TrimSpace(expectedRedirectURI)
	if clientID == "" || !validAdminBFFRedirect(expectedRedirectURI) {
		return AdminBFFRepairResult{}, fmt.Errorf("%w: client ID and an exact HTTPS Admin BFF callback are required", ErrAdminBFFRepairIneligible)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return AdminBFFRepairResult{}, fmt.Errorf("start admin BFF repair: %w", err)
	}
	defer tx.Rollback()

	var result AdminBFFRepairResult
	var appType, accessPolicy, secretHash string
	var enabled, internal int
	var archivedAt sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT id, name, application_type,
		access_policy, COALESCE(client_secret_hash, ''), enabled,
		internal, archived_at_unix
		FROM identity_applications WHERE client_id = ?`, clientID).
		Scan(&result.ApplicationID, &result.Name, &appType, &accessPolicy,
			&secretHash, &enabled, &internal, &archivedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AdminBFFRepairResult{}, ErrAdminBFFRepairNotFound
	}
	if err != nil {
		return AdminBFFRepairResult{}, fmt.Errorf("load admin BFF application: %w", err)
	}
	result.ClientID = clientID

	// These checks guard against accidentally turning an ordinary or public
	// application into privileged infrastructure.
	if appType != "confidential" || secretHash == "" ||
		accessPolicy != "default_deny" || enabled != 1 ||
		archivedAt.Valid || (internal != 0 && internal != 1) {
		return AdminBFFRepairResult{}, fmt.Errorf("%w: registration is not an enabled, deny-by-default confidential client", ErrAdminBFFRepairIneligible)
	}

	required := []struct {
		table  string
		column string
		value  string
	}{
		{"identity_application_scopes", "scope", "identity.admin"},
		{"identity_application_audiences", "audience", "mw-identity-admin"},
		{"identity_application_redirect_uris", "redirect_uri", expectedRedirectURI},
	}
	for _, item := range required {
		// Table and column identifiers are fixed constants from this function;
		// only the bound value comes from operator input.
		query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE application_id = ? AND %s = ?)", item.table, item.column)
		var present int
		if err := tx.QueryRowContext(ctx, query, result.ApplicationID, item.value).Scan(&present); err != nil {
			return AdminBFFRepairResult{}, fmt.Errorf("verify admin BFF registration: %w", err)
		}
		if present != 1 {
			return AdminBFFRepairResult{}, fmt.Errorf("%w: required scope, audience or exact callback missing", ErrAdminBFFRepairIneligible)
		}
	}

	result.AlreadyInternal = internal == 1
	if result.AlreadyInternal || !apply {
		return result, nil
	}

	update, err := tx.ExecContext(ctx, `UPDATE identity_applications
		SET internal = 1, updated_at_unix = ?
		WHERE id = ? AND client_id = ? AND internal = 0
		  AND enabled = 1 AND archived_at_unix IS NULL
		  AND application_type = 'confidential'
		  AND access_policy = 'default_deny'`,
		now.UTC().Unix(), result.ApplicationID, clientID)
	if err != nil {
		return AdminBFFRepairResult{}, fmt.Errorf("promote admin BFF: %w", err)
	}
	changed, err := update.RowsAffected()
	if err != nil || changed != 1 {
		return AdminBFFRepairResult{}, fmt.Errorf("admin BFF repair preconditions changed: %v", err)
	}

	auditID, err := auth.NewOpaqueToken("aud_")
	if err != nil {
		return AdminBFFRepairResult{}, fmt.Errorf("generate admin BFF repair audit ID: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"application_id": result.ApplicationID,
		"client_id":      result.ClientID,
		"changed_field":  "internal",
		"previous_value": false,
		"new_value":      true,
		"source":         "operator_cli",
	})
	if err != nil {
		return AdminBFFRepairResult{}, fmt.Errorf("encode admin BFF repair audit event: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identity_audit_events
		(id, event_type, status, severity, reason_code, metadata_json, created_at_unix)
		VALUES (?, 'identity.infrastructure.admin_bff_repaired', 'success',
		        'warning', 'operator_cli', ?, ?)`,
		auditID, string(metadata), now.UTC().Unix()); err != nil {
		return AdminBFFRepairResult{}, fmt.Errorf("record admin BFF repair audit event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return AdminBFFRepairResult{}, fmt.Errorf("commit admin BFF repair: %w", err)
	}
	result.Changed = true
	return result, nil
}

func validAdminBFFRedirect(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" &&
		u.User == nil && u.Fragment == "" && u.RawQuery == "" &&
		u.Path == "/api/auth/callback" && u.ForceQuery == false &&
		u.Opaque == ""
}
