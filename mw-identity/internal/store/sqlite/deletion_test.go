package sqlite

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func TestLegacyDeletionMigrationRequiresExplicitReconciliation(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "018_durable_deletion.sql" {
			continue
		}
		raw, err := fs.ReadFile(migrations.FS, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[entry.Name()] = &fstest.MapFile{Data: raw}
	}
	if err = ApplyMigrations(ctx, db, before); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO identity_users(id,email,email_normalized,display_name,password_hash,enabled,deletion_requested_at_unix,created_at_unix,updated_at_unix) VALUES ('legacy','l@example.test','l@example.test','Legacy','hash',0,1234,1,1); INSERT INTO identity_applications(id,name,client_id,application_type,created_at_unix,updated_at_unix) VALUES ('legacy-app','Legacy app','legacy-client','public',1,1); INSERT INTO identity_application_consents(user_id,application_id,created_at_unix,updated_at_unix) VALUES ('legacy','legacy-app',1,1)`); err != nil {
		t.Fatal(err)
	}
	if err = ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	ops, err := store.ListDeletionOperations(ctx)
	if err != nil || len(ops) != 1 || ops[0].Status != "legacy_review_required" || ops[0].RequestedAt.Unix() != 1234 {
		t.Fatalf("legacy request lost: %+v %v", ops, err)
	}
	if _, err = store.ClaimDeletionDelivery(ctx, time.Now(), "worker"); err == nil {
		t.Fatal("migration fabricated a delivery")
	}
	stats, err := store.DeletionQueueStats(ctx, time.Now())
	if err != nil || stats["pending_operations"] != 1 || stats["outstanding"] != 0 {
		t.Fatalf("operator-review work missing from metrics: %v, %v", stats, err)
	}
	if err = store.ReconcileLegacyDeletion(ctx, ops[0].ID, nil, "support-case", time.Now()); err == nil {
		t.Fatal("unverified legacy app finalized")
	}
	if err = store.ReconcileLegacyDeletion(ctx, ops[0].ID, map[string]string{"legacy-client": "no_current_mapping"}, "support-case", time.Now()); err != nil {
		t.Fatal(err)
	}
	op, err := store.DeletionOperation(ctx, ops[0].ID)
	if err != nil || op.Status != "completed" || len(op.Deliveries) != 1 || !strings.HasPrefix(op.Deliveries[0].EventID, "receipt_") {
		t.Fatalf("manual outcome did not finalize without replay: %+v %v", op, err)
	}
}

func TestDeletionAtomicRollbackAndBackupPreservePendingWork(t *testing.T) {
	f := newRemovalFixture(t, "https://app.example/delete")
	ctx := context.Background()
	if _, err := f.store.db.ExecContext(ctx, `CREATE TRIGGER reject_delivery BEFORE INSERT ON identity_deletion_deliveries BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.BeginDurableDeletion(ctx, "usr_remove", f.now, "hash", f.now.Add(time.Hour)); err == nil {
		t.Fatal("snapshot fault ignored")
	}
	user, err := f.store.IdentityUserByID(ctx, "usr_remove")
	if err != nil || !user.Enabled || user.DeletionRequestedAt != nil {
		t.Fatal("partial deletion committed")
	}
	if _, err = f.store.db.ExecContext(ctx, `DROP TRIGGER reject_delivery`); err != nil {
		t.Fatal(err)
	}
	op, err := f.store.BeginDurableDeletion(ctx, user.ID, f.now, "hash", f.now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "backup.db")
	if err = BackupDatabase(ctx, f.store.db, backup); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	copyStore := NewStore(restored)
	got, err := copyStore.DeletionOperation(ctx, op.ID)
	if err != nil || got.Status != "pending" || len(got.Deliveries) != 2 || got.Deliveries[0].EventID != op.Deliveries[0].EventID {
		t.Fatal("backup lost pending stable work")
	}
	if _, err = RunMaintenance(ctx, restored, f.now.Add(40*24*time.Hour), 24*time.Hour, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err = copyStore.DeletionOperation(ctx, op.ID); err != nil {
		t.Fatal("maintenance purged unresolved work")
	}
}

func deletionService(t *testing.T, store *Store) (*auth.Service, *signing.Manager) {
	t.Helper()
	ring, err := signing.ParseKeyring("1:"+base64.RawURLEncoding.EncodeToString(make([]byte, 32)), 1)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signing.NewManager(store, ring)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = signer.EnsureActiveKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	s, err := auth.NewService(store, store, auth.ServiceConfig{SessionTTL: time.Hour, PublicIssuer: "https://identity.example"})
	if err != nil {
		t.Fatal(err)
	}
	s.SetLifecycleSigner(signer)
	s.SetDeletionDispatch(true, false)
	return s, signer
}
func TestDurableDeletionSurvivesLostAcknowledgementAndRestart(t *testing.T) {
	ctx := context.Background()
	var lock sync.Mutex
	var events []string
	var proofs []string
	var signer *signing.Manager
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Error("no JWT")
			w.WriteHeader(401)
			return
		}
		raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var claims map[string]any
		if err := json.Unmarshal(raw, &claims); err != nil {
			t.Error(err)
		}
		if claims["reason"] != "account_deleted" || claims["jti"] != body["id"] || body["id"] != r.Header.Get("X-MW-Identity-Event-ID") || claims["operation_id"] != body["operation_id"] || claims["requested_at"] != body["requested_at"] || claims["aud"] != "one-client" {
			t.Error("signed bindings mismatch")
		}
		key, err := signer.ActiveSigner(ctx)
		if err != nil {
			t.Error(err)
		} else {
			signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
			digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			if err = rsa.VerifyPKCS1v15(&key.PrivateKey.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
				t.Error(err)
			}
		}
		lock.Lock()
		events = append(events, body["id"].(string))
		proofs = append(proofs, token)
		first := len(events) == 1
		lock.Unlock()
		if first {
			connection, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				connection.Close()
			}
			return
		}
		w.Write([]byte(`{"version":1,"outcome":"confirmed_cleanup"}`))
	}))
	defer callback.Close()
	f := newRemovalFixture(t, callback.URL)
	if _, err := f.store.db.ExecContext(ctx, `UPDATE identity_applications SET internal=1 WHERE id='app_two'`); err != nil {
		t.Fatal(err)
	}
	service, manager := deletionService(t, f.store)
	signer = manager
	result, err := service.DeleteAccount(ctx, "usr_remove", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Deleted || result.OperationID == "" || len(events) != 0 {
		t.Fatal("initiation performed callback or finalized")
	}
	user, err := f.store.IdentityUserByID(ctx, "usr_remove")
	if err != nil || user.Enabled || user.DeletionRequestedAt == nil {
		t.Fatal("identity not disabled atomically")
	}
	if _, err = f.store.db.ExecContext(ctx, `UPDATE identity_users SET enabled=1 WHERE id='usr_remove'`); err == nil {
		t.Fatal("deleting identity reenabled")
	}
	if err = f.store.CreateIdentitySession(ctx, auth.SessionRecord{ID: "late", UserID: user.ID, TokenHash: "late", CSRFHash: "late", CreatedAt: f.now, ExpiresAt: f.now.Add(time.Hour)}); err == nil {
		t.Fatal("stale authentication created a session")
	}
	again, err := service.DeleteAccount(ctx, user.ID, "")
	if err != nil || again.OperationID != result.OperationID || again.StatusToken != "" {
		t.Fatal("duplicate deletion created another operation")
	}
	if worked, err := service.ProcessDeletionDelivery(ctx); err != nil || !worked {
		t.Fatalf("delivery: %v %v", worked, err)
	}
	op, err := f.store.DeletionOperation(ctx, result.OperationID)
	if err != nil || op.Status != "pending" {
		t.Fatal("lost acknowledgement finalized")
	}
	// Restart the service with the persisted queue; rotate signing material between attempts.
	if _, err = manager.Rotate(ctx); err != nil {
		t.Fatal(err)
	}
	restarted, _ := deletionService(t, f.store)
	if _, err = f.store.db.ExecContext(ctx, `UPDATE identity_deletion_deliveries SET next_attempt_at_unix=0 WHERE status='pending'`); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.ProcessDeletionDelivery(ctx); err != nil {
		t.Fatal(err)
	}
	op, err = f.store.DeletionOperation(ctx, result.OperationID)
	if err != nil || op.Status != "completed" {
		t.Fatalf("not complete: %+v %v", op, err)
	}
	if _, err = f.store.IdentityUserByID(ctx, user.ID); err == nil {
		t.Fatal("central user retained after required cleanup")
	}
	if len(events) != 2 || events[0] != events[1] || proofs[0] == proofs[1] {
		t.Fatal("retry did not preserve event with refreshed signing proof")
	}
	if _, err = f.store.DeletionByCapability(ctx, auth.HashToken(result.StatusToken), time.Now()); err != nil {
		t.Fatal("status lost with user cascade")
	}
	if _, err = f.store.DeletionByCapability(ctx, auth.HashToken(result.OperationID), time.Now()); err == nil {
		t.Fatal("operation ID authorized public status")
	}
}

func TestDeletionLeaseRecoveryAndCrashBeforeFinalization(t *testing.T) {
	f := newRemovalFixture(t, "https://app.example/delete")
	ctx := context.Background()
	if _, err := f.store.db.ExecContext(ctx, `UPDATE identity_applications SET internal=1 WHERE id='app_two'`); err != nil {
		t.Fatal(err)
	}
	op, err := f.store.BeginDurableDeletion(ctx, "usr_remove", f.now, "hash", f.now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	first, err := f.store.ClaimDeletionDelivery(ctx, f.now, "old-worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.ClaimDeletionDelivery(ctx, f.now, "parallel-worker"); err == nil {
		t.Fatal("duplicate active claim")
	}
	recovered, err := f.store.ClaimDeletionDelivery(ctx, f.now.Add(31*time.Second), "new-worker")
	if err != nil || recovered.EventID != first.EventID {
		t.Fatal("lease not recovered")
	}
	if err = f.store.FinishDeletionDelivery(ctx, first, "confirmed_cleanup", "", f.now); err == nil {
		t.Fatal("stale worker committed result")
	}
	if err = f.store.FinishDeletionDelivery(ctx, recovered, "confirmed_cleanup", "", f.now); err != nil {
		t.Fatal(err)
	}
	// Process exited between acknowledgement persistence and final deletion.
	if err = f.store.FinalizeReadyDeletions(ctx, f.now); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.DeletionOperation(ctx, op.ID)
	if err != nil || got.Status != "completed" {
		t.Fatal("completion lost after restart")
	}
}

func TestQueuedCallbackAndChangedRegistrationRemainPending(t *testing.T) {
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/complete" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":1,"outcome":"no_current_mapping"}`))
			return
		}
		w.WriteHeader(202)
	}))
	defer callback.Close()
	f := newRemovalFixture(t, callback.URL)
	ctx := context.Background()
	service, _ := deletionService(t, f.store)
	// Callback-less legacy app is unresolved, not verified erasure.
	result, err := service.DeleteAccount(ctx, "usr_remove", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProcessDeletionDelivery(ctx); err != nil {
		t.Fatal(err)
	}
	op, err := f.store.DeletionOperation(ctx, result.OperationID)
	if err != nil || op.Status != "pending" {
		t.Fatal("202 finalized deletion")
	}
	if _, err = f.store.db.ExecContext(ctx, `UPDATE identity_applications SET delete_callback_uri=? WHERE id='app_one'; UPDATE identity_deletion_deliveries SET next_attempt_at_unix=0 WHERE status='pending'`, callback.URL+"/complete"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProcessDeletionDelivery(ctx); err != nil {
		t.Fatal(err)
	}
	op, err = f.store.DeletionOperation(ctx, result.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	var event string
	for _, d := range op.Deliveries {
		if d.ApplicationID == "app_one" {
			event = d.EventID
			if d.Status != "unresolved" {
				t.Fatal("callback change silently followed")
			}
		}
	}
	if err = f.store.RetryDeletionDelivery(ctx, event, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProcessDeletionDelivery(ctx); err != nil {
		t.Fatal(err)
	}
	op, err = f.store.DeletionOperation(ctx, result.OperationID)
	if err != nil || op.Status != "pending" {
		t.Fatal("callback-less app silently completed")
	}
	for _, d := range op.Deliveries {
		if d.ApplicationID == "app_one" && d.Status != "no_current_mapping" {
			t.Fatal("missing differentiated no-mapping outcome")
		}
	}
}

func TestDeletionRedirectIsNotFollowedOrAcknowledged(t *testing.T) {
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte(`{"version":1,"outcome":"confirmed_cleanup"}`))
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer source.Close()
	f := newRemovalFixture(t, source.URL)
	service, _ := deletionService(t, f.store)
	ctx := context.Background()
	result, err := service.DeleteAccount(ctx, "usr_remove", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProcessDeletionDelivery(ctx); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("lifecycle proof sent to redirected target")
	}
	op, err := f.store.DeletionOperation(ctx, result.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	for _, delivery := range op.Deliveries {
		if delivery.ApplicationID == "app_one" && (delivery.Status != "unresolved" || delivery.ErrorCode != "callback_redirect") {
			t.Fatalf("redirect falsely completed/retried: %+v", delivery)
		}
	}
}

func TestOperatorRetryRenewsHorizonWithoutChangingEvent(t *testing.T) {
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":1,"outcome":"confirmed_cleanup"}`))
	}))
	defer callback.Close()
	f := newRemovalFixture(t, callback.URL)
	ctx := context.Background()
	service, _ := deletionService(t, f.store)
	if _, err := f.store.db.ExecContext(ctx, `UPDATE identity_applications SET internal=1 WHERE id='app_two'`); err != nil {
		t.Fatal(err)
	}
	result, err := service.DeleteAccount(ctx, "usr_remove", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.db.ExecContext(ctx, `UPDATE identity_deletion_deliveries SET attempts=1,next_attempt_at_unix=0,retry_started_at_unix=? WHERE status='pending'`, time.Now().Add(-8*24*time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProcessDeletionDelivery(ctx); err != nil {
		t.Fatal(err)
	}
	op, err := f.store.DeletionOperation(ctx, result.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	var event string
	for _, d := range op.Deliveries {
		if d.ApplicationID == "app_one" {
			event = d.EventID
			if d.Status != "failed" || d.ErrorCode != "retry_horizon" {
				t.Fatal("exhausted delivery did not require recovery")
			}
		}
	}
	if err = f.store.RetryDeletionDelivery(ctx, event, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProcessDeletionDelivery(ctx); err != nil {
		t.Fatal(err)
	}
	op, err = f.store.DeletionOperation(ctx, result.OperationID)
	if err != nil || op.Status != "completed" {
		t.Fatal("operator retry could not recover after horizon")
	}
	for _, d := range op.Deliveries {
		if d.ApplicationID == "app_one" && d.EventID != event {
			t.Fatal("operator retry changed idempotency key")
		}
	}
}

func TestDeletionCallbackRequiresTerminalAcknowledgement(t *testing.T) {
	for _, tc := range []struct {
		name, body, outcome string
		code                int
	}{
		{"empty 200 rejected", "", "pending", 200},
		{"empty 204 rejected", "", "pending", 204},
		{"unversioned cleanup rejected", `{"outcome":"confirmed_cleanup"}`, "pending", 200},
		{"unversioned no mapping rejected", `{"outcome":"no_current_mapping"}`, "pending", 200},
		{"versioned cleanup", `{"version":1,"outcome":"confirmed_cleanup"}`, "confirmed_cleanup", 200},
		{"versioned no mapping", `{"version":1,"outcome":"no_current_mapping"}`, "no_current_mapping", 200},
		{"accepted is pending", `{"version":1,"outcome":"confirmed_cleanup"}`, "pending", 202},
		{"pending", `{"outcome":"pending"}`, "pending", 200},
		{"versioned pending", `{"version":1,"outcome":"pending"}`, "pending", 200},
		{"unknown outcome", `{"outcome":"unknown"}`, "pending", 200},
		{"unknown version", `{"version":2,"outcome":"confirmed_cleanup"}`, "pending", 200},
		{"null version", `{"version":null,"outcome":"confirmed_cleanup"}`, "pending", 200},
		{"missing outcome", `{"version":1}`, "pending", 200},
		{"empty object", `{}`, "pending", 200},
		{"whitespace body", " \n\t", "pending", 200},
		{"malformed", `{"outcome":`, "pending", 200},
		{"non JSON", `pending`, "pending", 200},
		{"null envelope", `null`, "pending", 200},
		{"array envelope", `[]`, "pending", 200},
		{"wrong outcome type", `{"outcome":true}`, "pending", 200},
		{"trailing JSON", `{"outcome":"confirmed_cleanup"}{}`, "pending", 200},
		{"duplicate outcome", `{"outcome":"pending","outcome":"confirmed_cleanup"}`, "pending", 200},
		{"unknown field", `{"version":1,"outcome":"confirmed_cleanup","pending":true}`, "pending", 200},
		{"oversized body", strings.Repeat("x", 32*1024+1), "pending", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer callback.Close()
			f := newRemovalFixture(t, callback.URL)
			ctx := context.Background()
			if _, err := f.store.db.ExecContext(ctx, `UPDATE identity_applications SET internal=1 WHERE id='app_two'`); err != nil {
				t.Fatal(err)
			}
			f.service.SetDeletionDispatch(true, false)
			result, err := f.service.DeleteAccount(ctx, "usr_remove", "")
			if err != nil {
				t.Fatal(err)
			}
			if worked, err := f.service.ProcessDeletionDelivery(ctx); err != nil || !worked {
				t.Fatalf("worker: %v %v", worked, err)
			}
			op, err := f.store.DeletionOperation(ctx, result.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			for _, delivery := range op.Deliveries {
				if delivery.ApplicationID == "app_one" && delivery.Status != tc.outcome {
					t.Fatalf("delivery = %+v, want %s", delivery, tc.outcome)
				}
			}
			if tc.outcome == "pending" {
				user, err := f.store.IdentityUserByID(ctx, "usr_remove")
				if op.Status != "pending" || err != nil || user.Enabled || user.DeletionRequestedAt == nil {
					t.Fatalf("nonterminal acknowledgement finalized or reopened user: %+v %+v %v", op, user, err)
				}
			} else if op.Status != "completed" {
				t.Fatalf("terminal acknowledgement did not complete: %+v", op)
			}
		})
	}
}

func TestDeletionFinalizationFailsClosedDuringRecovery(t *testing.T) {
	for _, tc := range []struct{ name, damage string }{
		{"missing subject", `DELETE FROM identity_users WHERE id='usr_remove'`},
		{"missing marker", `UPDATE identity_users SET deletion_requested_at_unix=NULL WHERE id='usr_remove'`},
		{"restored active subject", `UPDATE identity_users SET enabled=1,deletion_requested_at_unix=NULL WHERE id='usr_remove'`},
		{"delete ignored", `CREATE TRIGGER ignore_user_delete BEFORE DELETE ON identity_users BEGIN SELECT RAISE(IGNORE); END`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRemovalFixture(t, "https://app.example/delete")
			ctx := context.Background()
			op, err := f.store.BeginDurableDeletion(ctx, "usr_remove", f.now, "hash", f.now.Add(time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.store.db.ExecContext(ctx, `UPDATE identity_deletion_deliveries SET status='confirmed_cleanup' WHERE operation_id=?`, op.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = f.store.db.ExecContext(ctx, tc.damage); err != nil {
				t.Fatal(err)
			}
			if err = f.store.FinalizeDurableDeletion(ctx, op.ID, f.now); err == nil {
				t.Fatal("inconsistent recovery state finalized")
			}
			if err = f.store.FinalizeReadyDeletions(ctx, f.now); err == nil {
				t.Fatal("restart sweep finalized inconsistent recovery state")
			}
			var status string
			var completed *int64
			if err = f.store.db.QueryRowContext(ctx, `SELECT status,completed_at_unix FROM identity_deletion_operations WHERE id=?`, op.ID).Scan(&status, &completed); err != nil || status != "pending" || completed != nil {
				t.Fatalf("false completion persisted: %s %v %v", status, completed, err)
			}
		})
	}
}

func TestOperatorRequestsAndRetriesPreserveBrowserProgressCredential(t *testing.T) {
	f := newRemovalFixture(t, "https://app.example/delete")
	ctx := context.Background()
	f.service.SetDeletionDispatch(true, false)
	result, err := f.service.DeleteAccount(ctx, "usr_remove", "")
	if err != nil || result.StatusToken == "" {
		t.Fatalf("missing initiating credential: %+v %v", result, err)
	}
	var originalHash string
	var originalExpiry int64
	if err := f.store.db.QueryRowContext(ctx, `SELECT capability_hash,capability_expires_at_unix FROM identity_deletion_operations WHERE id=?`, result.OperationID).Scan(&originalHash, &originalExpiry); err != nil {
		t.Fatal(err)
	}
	// The user-delete admin API repeats DeleteAccount while the subject is frozen.
	repeated, err := f.service.DeleteAccount(ctx, "usr_remove", "")
	if err != nil || repeated.StatusToken != "" || repeated.OperationID != result.OperationID {
		t.Fatalf("retry issued a replacement credential: %+v %v", repeated, err)
	}
	op, err := f.store.DeletionOperation(ctx, result.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	for _, delivery := range op.Deliveries {
		if delivery.ApplicationID == "app_one" {
			if err := f.store.RetryDeletionDelivery(ctx, delivery.EventID, false, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
		}
	}
	var hash string
	var expiry int64
	if err := f.store.db.QueryRowContext(ctx, `SELECT capability_hash,capability_expires_at_unix FROM identity_deletion_operations WHERE id=?`, op.ID).Scan(&hash, &expiry); err != nil {
		t.Fatal(err)
	}
	if hash != originalHash || expiry != originalExpiry {
		t.Fatal("operator retry rotated or extended the user's credential")
	}
	if _, err := f.store.DeletionByCapability(ctx, auth.HashToken(result.StatusToken), time.Now()); err != nil {
		t.Fatal("original browser progress credential invalidated")
	}
	if _, err := f.store.DeletionByCapability(ctx, auth.HashToken(result.OperationID), time.Now()); err == nil {
		t.Fatal("operation ID authorized progress")
	}
	if _, err := f.store.DeletionByCapability(ctx, originalHash, time.Unix(originalExpiry, 0)); err == nil {
		t.Fatal("operator retry extended credential expiry")
	}
	// Operator-verified completion retains the same public progress credential.
	if _, err := f.store.db.ExecContext(ctx, `UPDATE identity_deletion_deliveries SET status='no_current_mapping' WHERE operation_id=?`, op.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.store.FinalizeDurableDeletion(ctx, op.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	completed, err := f.store.DeletionByCapability(ctx, originalHash, time.Now())
	if err != nil || completed.Status != "completed" {
		t.Fatal("progress credential did not survive finalization")
	}
}

func TestDispatchPauseNeverRestoresRequestBoundDeletion(t *testing.T) {
	for _, infrastructure := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary missing callback", true: "infrastructure exempt"}[infrastructure], func(t *testing.T) {
			calls := 0
			callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Write([]byte(`{"version":1,"outcome":"confirmed_cleanup"}`))
			}))
			defer callback.Close()
			f := newRemovalFixture(t, callback.URL)
			ctx := context.Background()
			if infrastructure {
				if _, err := f.store.db.ExecContext(ctx, `UPDATE identity_applications SET internal=1 WHERE id='app_two'`); err != nil {
					t.Fatal(err)
				}
			}
			f.service.SetDeletionDispatch(false, false)
			result, err := f.service.DeleteAccount(ctx, "usr_remove", "")
			if err != nil || result.Deleted || result.OperationID == "" || result.StatusToken == "" || result.Status != "pending" || calls != 0 {
				t.Fatalf("paused dispatch escaped durable workflow: %+v calls=%d err=%v", result, calls, err)
			}
			user, err := f.store.IdentityUserByID(ctx, "usr_remove")
			if err != nil || user.Enabled || user.DeletionRequestedAt == nil {
				t.Fatal("queued identity was not frozen")
			}
			op, err := f.store.DeletionOperation(ctx, result.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			for _, delivery := range op.Deliveries {
				if delivery.ApplicationID == "app_two" && delivery.Status != map[bool]string{false: "unresolved", true: "exempt"}[infrastructure] {
					t.Fatal("missing callback falsely completed")
				}
			}
			f.service.SetDeletionDispatch(true, false)
			if _, err := f.service.ProcessDeletionDelivery(ctx); err != nil {
				t.Fatal(err)
			}
			op, err = f.store.DeletionOperation(ctx, result.OperationID)
			if err != nil || (op.Status == "completed") != infrastructure || calls != 1 {
				t.Fatalf("resume did not honor durable obligations: %+v calls=%d err=%v", op, calls, err)
			}
		})
	}
}
