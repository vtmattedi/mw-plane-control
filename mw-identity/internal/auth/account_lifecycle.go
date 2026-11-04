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

// Only whole-MW-account deletion emits a destructive lifecycle callback.
const DeletionReasonAccountDeleted = "account_deleted"

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

	if s.durableDeletion || user.DeletionRequestedAt != nil {
		return s.beginDurableDeletion(ctx, userID)
	}
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

// RemoveConnectedApplication revokes sign-in consent, preserving app data and assignments.
func (s *Service) RemoveConnectedApplication(ctx context.Context, userID, applicationID, remoteIP string) (DeletionDelivery, error) {
	apps, err := s.store.ListIdentityUserDeletionApplications(ctx, userID)
	if err != nil {
		return DeletionDelivery{}, err
	}
	for _, app := range apps {
		if app.ID != applicationID {
			continue
		}
		if err := s.store.RevokeIdentityUserApplicationAccess(ctx, userID, app.ClientID); err != nil {
			return DeletionDelivery{}, err
		}
		s.recordAudit(ctx, audit.Event{ID: mustAuditID(), EventType: "identity.application.disconnected", Status: "success", Severity: "info", UserID: &userID, RemoteIP: remoteIP, CreatedAt: s.now().UTC()})
		return DeletionDelivery{ApplicationID: app.ID, ClientID: app.ClientID, Status: "disconnected"}, nil
	}
	return DeletionDelivery{}, ErrNotFound
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
		"iss":    s.publicIssuer,
		"sub":    userID,
		"aud":    app.ClientID,
		"iat":    requestedAt.Unix(),
		"exp":    requestedAt.Add(5 * time.Minute).Unix(),
		"jti":    eventID,
		"event":  "identity.user.deleted",
		"reason": reason,
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
				raw, readErr := io.ReadAll(io.LimitReader(response.Body, 32*1024+1))
				_ = response.Body.Close()
				if readErr == nil && len(raw) <= 32*1024 {
					outcome, _ := deletionResponseOutcome(response.StatusCode, raw)
					if outcome == "confirmed_cleanup" || outcome == "no_current_mapping" {
						result.Status = "delivered"
						return result
					}
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
