package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/applications"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

func TestDeletionMetricsExposePausedPendingOperationsUntilFinalization(t *testing.T) {
	service, h, store := testAuthHTTPWithStore(t)
	ctx := context.Background()
	assertMetrics := func(enabled, pending, outstanding, paused int64, secrets ...string) {
		t.Helper()
		rec := httptest.NewRecorder()
		h.deletionMetrics(rec, httptest.NewRequest("GET", "/metrics/deletions", nil))
		if rec.Code != 200 {
			t.Fatalf("metrics status = %d: %s", rec.Code, rec.Body.String())
		}
		var got map[string]int64
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]int64{
			"dispatch_enabled": enabled, "pending_operations": pending,
			"outstanding": outstanding, "dispatch_paused_with_pending": paused,
			"failed_or_unresolved": 0,
		} {
			if value, ok := got[key]; !ok || value != want {
				t.Fatalf("%s = %d (present %v), want %d", key, value, ok, want)
			}
		}
		if age, ok := got["oldest_age_seconds"]; !ok || age < 0 {
			t.Fatalf("invalid queue age: %v", got)
		}
		for _, secret := range secrets {
			if secret != "" && strings.Contains(rec.Body.String(), secret) {
				t.Fatal("metrics exposed an identity, operation or credential")
			}
		}
	}
	assertMetrics(1, 0, 0, 0)
	service.SetDeletionDispatch(false, false)
	assertMetrics(0, 0, 0, 0)

	user, err := service.RegisterPublic(ctx, auth.RegisterPublicRequest{Email: "paused@example.test", Username: "paused.user", Password: []byte("a-long-test-password")}, "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err = store.CreateApplication(ctx, applications.CreateInput{ID: "metrics-app", Name: "Metrics app", ClientID: "metrics-client", ApplicationType: "public", AccessPolicy: "default_allow", Enabled: true, DeleteCallbackURI: "https://app.example/delete", Scopes: []string{"openid"}, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err = store.GrantIdentityConsent(ctx, user.ID, "metrics-app", []string{"openid"}, 0, now); err != nil {
		t.Fatal(err)
	}
	op, err := service.DeleteAccount(ctx, user.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	assertMetrics(0, 1, 1, 1, user.ID, op.OperationID, op.StatusToken)
	service.SetDeletionDispatch(true, false)
	assertMetrics(1, 1, 1, 0)
	service.SetDeletionDispatch(false, false)

	delivery, err := store.ClaimDeletionDelivery(ctx, time.Now().UTC(), "metrics-test-lease")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.FinishDeletionDelivery(ctx, delivery, "confirmed_cleanup", "", now); err != nil {
		t.Fatal(err)
	}
	// A saved receiver receipt is not an operation completion. A paused worker
	// can still leave finalization pending even with no outstanding deliveries.
	assertMetrics(0, 1, 0, 1)
	if err = store.FinalizeDurableDeletion(ctx, op.OperationID, now); err != nil {
		t.Fatal(err)
	}
	assertMetrics(0, 0, 0, 0)
}
