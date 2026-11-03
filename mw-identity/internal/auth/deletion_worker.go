package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"
)

func (s *Service) SetDurableDeletion(enabled, production bool) {
	s.durableDeletion = enabled
	if production {
		s.httpClient = productionDeletionClient()
	}
}
func (s *Service) DurableDeletionStore() (DurableDeletionStore, error) {
	store, ok := s.store.(DurableDeletionStore)
	if !ok {
		return nil, errors.New("durable deletion unavailable")
	}
	return store, nil
}
func (s *Service) beginDurableDeletion(ctx context.Context, user string) (DeleteAccountResult, error) {
	store, err := s.DurableDeletionStore()
	if err != nil {
		return DeleteAccountResult{}, err
	}
	capability, err := NewOpaqueToken("deletion_status_")
	if err != nil {
		return DeleteAccountResult{}, err
	}
	now := s.now().UTC()
	op, err := store.BeginDurableDeletion(ctx, user, now, HashToken(capability), now.Add(24*time.Hour))
	if err != nil {
		return DeleteAccountResult{}, err
	}
	result := DeleteAccountResult{Deleted: op.Status == "completed", OperationID: op.ID, Status: op.Status, StatusToken: capability, Deliveries: []DeletionDelivery{}}
	for _, d := range op.Deliveries {
		result.Deliveries = append(result.Deliveries, DeletionDelivery{ApplicationID: d.ApplicationID, ClientID: d.ClientID, Status: d.Status, Attempts: d.Attempts})
	}
	return result, nil
}

// RunDeletionWorker blocks until shutdown and waits for every bounded worker.
func (s *Service) RunDeletionWorker(ctx context.Context) {
	if !s.durableDeletion {
		return
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timer := time.NewTicker(time.Second)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					if _, err := s.ProcessDeletionDelivery(ctx); err != nil && ctx.Err() == nil {
						slog.Error("deletion worker storage failure")
					}
				}
			}
		}()
	}
	wg.Wait()
}

// ProcessDeletionDelivery also supports isolated deterministic runtime checks.
func (s *Service) ProcessDeletionDelivery(ctx context.Context) (bool, error) {
	store, err := s.DurableDeletionStore()
	if err != nil {
		return false, err
	}
	lease, err := NewOpaqueToken("lease_")
	if err != nil {
		return false, err
	}
	now := s.now().UTC()
	d, err := store.ClaimDeletionDelivery(ctx, now, lease)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, store.FinalizeReadyDeletions(ctx, now)
		}
		return false, err
	}
	status, category := "pending", "callback_unavailable"
	if d.ErrorCode != "" {
		status, category = "unresolved", d.ErrorCode
	} else if d.Attempts > 1 && now.Sub(d.RetryStartedAt) > 7*24*time.Hour {
		status, category = "failed", "retry_horizon"
	} else {
		status, category = s.deliverDurableDeletion(ctx, d, now)
	}
	delay := time.Second * time.Duration(1<<min(d.Attempts, 12))
	delay = min(delay, time.Hour)
	delay = min(time.Hour, delay+time.Duration(rand.Int64N(int64(delay/4)+1)))
	// Persist using a separate bounded context: shutdown leaves a retryable lease if persistence fails.
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err = store.FinishDeletionDelivery(persist, d, status, category, now.Add(delay)); err != nil {
		return true, err
	}
	return true, store.FinalizeDurableDeletion(persist, d.OperationID, now)
}
func (s *Service) deliverDurableDeletion(ctx context.Context, d DurableDelivery, now time.Time) (string, string) {
	if s.lifecycleSigner == nil {
		return "pending", "signing_unavailable"
	}
	requested := d.RequestedAt.Format(time.RFC3339)
	body := map[string]any{"id": d.EventID, "operation_id": d.OperationID, "type": "identity.user.deleted", "sub": d.Subject, "application_id": d.ApplicationID, "client_id": d.ClientID, "reason": DeletionReasonAccountDeleted, "requested_at": requested}
	payload, err := json.Marshal(body)
	if err != nil {
		return "unresolved", "invalid_snapshot"
	}
	assertion, err := s.lifecycleSigner.SignJWT(ctx, map[string]any{"iss": s.publicIssuer, "sub": d.Subject, "aud": d.ClientID, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "jti": d.EventID, "event": "identity.user.deleted", "reason": DeletionReasonAccountDeleted, "operation_id": d.OperationID, "application_id": d.ApplicationID, "requested_at": requested})
	if err != nil {
		return "pending", "signing_unavailable"
	}
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, d.CallbackURI, bytes.NewReader(payload))
	if err != nil {
		return "unresolved", "invalid_callback"
	}
	request.Header.Set("Authorization", "Bearer "+assertion)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-MW-Identity-Event-ID", d.EventID)
	request.Header.Set("User-Agent", "mw-identity-account-lifecycle/1.0")
	response, err := s.httpClient.Do(request)
	if err != nil {
		if response != nil && response.StatusCode >= 300 && response.StatusCode < 400 {
			return "unresolved", "callback_redirect"
		}
		return "pending", "callback_unavailable"
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 32*1024+1))
	if err != nil || len(raw) > 32*1024 {
		return "pending", "invalid_response"
	}
	if response.StatusCode == http.StatusNoContent {
		return "confirmed_cleanup", ""
	}
	if response.StatusCode == http.StatusOK {
		var result struct {
			Outcome string `json:"outcome"`
		}
		if len(raw) > 0 && json.Unmarshal(raw, &result) == nil && result.Outcome == "no_current_mapping" {
			return "no_current_mapping", ""
		}
		return "confirmed_cleanup", ""
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		return "unresolved", "callback_redirect"
	}
	return "pending", "callback_not_completed"
}

func (s *Service) AuditDeletionRetry(ctx context.Context, actor, event, remote string, refresh bool) {
	s.recordAudit(ctx, audit.Event{ID: mustAuditID(), EventType: "identity.deletion.operator_retry", Status: "success", Severity: "warning", UserID: &actor, RemoteIP: remote, Metadata: map[string]any{"event_id": event, "refresh_snapshot": refresh}, CreatedAt: s.now().UTC()})
}
