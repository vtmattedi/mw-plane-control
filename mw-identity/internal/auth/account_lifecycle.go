package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
)

var deletionRetryDelays = []time.Duration{250 * time.Millisecond, 750 * time.Millisecond}

// Why an application receives identity.user.deleted. Either way the application
// must delete or anonymize its local data for the subject.
const (
	DeletionReasonAccountDeleted          = "account_deleted"
	DeletionReasonApplicationDisconnected = "application_disconnected"
)

func (s *Service) ConnectedApplications(ctx context.Context, userID string) ([]ConnectedApplication, error) {
	return s.store.ListIdentityUserApplications(ctx, userID)
}

func (s *Service) DeleteAccount(ctx context.Context, userID, remoteIP string) (DeleteAccountResult, error) {
	user, err := s.store.IdentityUserByID(ctx, userID)
	if err != nil {
		return DeleteAccountResult{}, err
	}
	if user.IsSuperadmin {
		return DeleteAccountResult{}, ErrAccountDeletionForbidden
	}

	apps, err := s.store.ListIdentityUserApplications(ctx, userID)
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
		if delivery.Status == "failed" {
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
			"deliveries": deliveries,
		},
		CreatedAt: now,
	})

	if !allDelivered {
		return DeleteAccountResult{Deleted: false, Deliveries: deliveries}, nil
	}
	if err := s.store.DeleteIdentityUser(ctx, userID); err != nil {
		return DeleteAccountResult{}, err
	}
	return DeleteAccountResult{Deleted: true, Deliveries: deliveries}, nil
}

// RemoveConnectedApplication disconnects one application from the account. Its
// codes and tokens stop working at once, the application is asked to delete the
// user's local data, and the assignment is removed only after it confirms (or
// when it has no deletion callback). If delivery fails, access stays revoked and
// the user can retry.
func (s *Service) RemoveConnectedApplication(ctx context.Context, userID, applicationID, remoteIP string) (DeletionDelivery, error) {
	apps, err := s.store.ListIdentityUserApplications(ctx, userID)
	if err != nil {
		return DeletionDelivery{}, err
	}
	var app ConnectedApplication
	found := false
	for _, candidate := range apps {
		if candidate.ID == applicationID {
			app, found = candidate, true
			break
		}
	}
	if !found {
		return DeletionDelivery{}, ErrNotFound
	}

	now := s.now().UTC()
	if err := s.store.RevokeIdentityUserApplicationAccess(ctx, userID, app.ClientID); err != nil {
		return DeletionDelivery{}, err
	}
	delivery := s.deliverDeletion(ctx, userID, app, now, DeletionReasonApplicationDisconnected)
	completed := delivery.Status != "failed"

	status := map[bool]string{true: "completed", false: "pending"}[completed]
	s.recordAudit(ctx, audit.Event{
		ID:         mustAuditID(),
		EventType:  "identity.application.disconnect_" + status,
		Status:     map[bool]string{true: "success", false: "failure"}[completed],
		Severity:   map[bool]string{true: "info", false: "warning"}[completed],
		UserID:     &userID,
		RemoteIP:   remoteIP,
		ReasonCode: DeletionReasonApplicationDisconnected,
		Metadata: map[string]any{
			"application_id": app.ID,
			"client_id":      app.ClientID,
			"delivery":       delivery,
		},
		CreatedAt: now,
	})

	if !completed {
		return delivery, nil
	}
	if err := s.store.RemoveIdentityUserAssignment(ctx, app.ID, userID); err != nil {
		return DeletionDelivery{}, err
	}
	return delivery, nil
}

func (s *Service) deliverDeletion(ctx context.Context, userID string, app ConnectedApplication, requestedAt time.Time, reason string) DeletionDelivery {
	result := DeletionDelivery{
		ApplicationID: app.ID,
		ClientID:      app.ClientID,
		Status:        "skipped",
	}
	if app.DeleteCallbackURI == "" {
		return result
	}

	eventID, err := NewOpaqueToken("evt_")
	if err != nil {
		result.Status = "failed"
		return result
	}
	payload, err := json.Marshal(map[string]any{
		"id":             eventID,
		"type":           "identity.user.deleted",
		"sub":            userID,
		"application_id": app.ID,
		"client_id":      app.ClientID,
		"reason":         reason,
		"requested_at":   requestedAt.Format(time.RFC3339),
	})
	if err != nil {
		result.Status = "failed"
		return result
	}
	if s.lifecycleSigner == nil {
		result.Status = "failed"
		return result
	}
	assertion, err := s.lifecycleSigner.SignJWT(ctx, map[string]any{
		"iss":   s.publicIssuer,
		"sub":   userID,
		"aud":   app.ClientID,
		"iat":   requestedAt.Unix(),
		"exp":   requestedAt.Add(5 * time.Minute).Unix(),
		"jti":   eventID,
		"event": "identity.user.deleted",
	})
	if err != nil {
		result.Status = "failed"
		return result
	}

	for attempt := 1; attempt <= 3; attempt++ {
		result.Attempts = attempt
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, app.DeleteCallbackURI, bytes.NewReader(payload))
		if requestErr == nil {
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("User-Agent", "mw-identity-account-lifecycle/1.0")
			request.Header.Set("X-MW-Identity-Event-ID", eventID)
			request.Header.Set("Authorization", "Bearer "+assertion)
			response, doErr := s.httpClient.Do(request)
			if doErr == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32*1024))
				_ = response.Body.Close()
				if response.StatusCode >= 200 && response.StatusCode < 300 {
					result.Status = "delivered"
					return result
				}
			}
		}
		if attempt < 3 {
			select {
			case <-ctx.Done():
				result.Status = "failed"
				return result
			case <-time.After(deletionRetryDelays[attempt-1]):
			}
		}
	}

	result.Status = "failed"
	return result
}

func deletionHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("delete callback redirects are not allowed")
		},
	}
}
