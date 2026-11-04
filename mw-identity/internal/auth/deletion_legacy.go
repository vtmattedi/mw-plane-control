package auth

import (
	"context"
	"sync"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
)

func (s *Service) deleteAccountLegacy(ctx context.Context, userID, remoteIP string) (DeleteAccountResult, error) {
	apps, err := s.store.ListIdentityUserDeletionApplications(ctx, userID)
	if err != nil {
		return DeleteAccountResult{}, err
	}
	now := s.now().UTC()
	if err := s.store.BeginIdentityUserDeletion(ctx, userID, now); err != nil {
		return DeleteAccountResult{}, err
	}

	deliveries := make([]DeletionDelivery, len(apps))
	var wg sync.WaitGroup
	for index, app := range apps {
		index, app := index, app
		wg.Add(1)
		go func() {
			defer wg.Done()
			deliveries[index] = s.deliverDeletion(ctx, userID, app, now, DeletionReasonAccountDeleted)
		}()
	}
	wg.Wait()

	allDelivered := true
	for _, delivery := range deliveries {
		if !DeletionOutcomeIsTerminal(delivery.Status) {
			allDelivered = false
			break
		}
	}

	status := "pending"
	if allDelivered {
		status = "completed"
	}
	s.recordAudit(ctx, audit.Event{
		ID:         mustAuditID(),
		EventType:  "identity.user.deletion_" + status,
		Status:     map[bool]string{true: "success", false: "failure"}[allDelivered],
		Severity:   map[bool]string{true: "info", false: "warning"}[allDelivered],
		UserID:     &userID,
		RemoteIP:   remoteIP,
		ReasonCode: "account_deletion",
		Metadata: map[string]any{
			"deliveries":    deliveries,
			"delivery_mode": "legacy_request_bound",
			"cleanup_scope": "known_applications_only",
		},
		CreatedAt: now,
	})

	if !allDelivered {
		return DeleteAccountResult{Deleted: false, Status: "legacy_pending", Deliveries: deliveries}, nil
	}
	if err := s.store.DeleteIdentityUser(ctx, userID); err != nil {
		return DeleteAccountResult{}, err
	}
	return DeleteAccountResult{Deleted: true, Status: "legacy_completed", Deliveries: deliveries}, nil
}
