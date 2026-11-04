package auth

import (
	"context"
	"errors"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
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
	if !op.StatusCredentialIssued {
		result.StatusToken = ""
	}
	for _, d := range op.Deliveries {
		result.Deliveries = append(result.Deliveries, DeletionDelivery{ApplicationID: d.ApplicationID, ClientID: d.ClientID, Status: d.Status, Attempts: d.Attempts})
	}
	return result, nil
}

func (s *Service) AuditDeletionRetry(ctx context.Context, actor, event, remote string, refresh bool) {
	s.recordAudit(ctx, audit.Event{ID: mustAuditID(), EventType: "identity.deletion.operator_retry", Status: "success", Severity: "warning", UserID: &actor, RemoteIP: remote, Metadata: map[string]any{"event_id": event, "refresh_snapshot": refresh}, CreatedAt: s.now().UTC()})
}
