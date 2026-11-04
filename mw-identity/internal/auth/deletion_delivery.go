package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var deletionRetryDelays = []time.Duration{250 * time.Millisecond, 750 * time.Millisecond}

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
	return s.sendDeletionCallback(ctx, d.CallbackURI, d.EventID, payload, assertion)
}

func (s *Service) deliverDeletion(ctx context.Context, userID string, app ConnectedApplication, requestedAt time.Time, reason string) DeletionDelivery {
	result := DeletionDelivery{
		ApplicationID: app.ID,
		ClientID:      app.ClientID,
		Status:        InitialDeletionStatus(app.Internal, app.DeleteCallbackURI),
	}
	if result.Status != "pending" {
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
		outcome, _ := s.sendDeletionCallback(ctx, app.DeleteCallbackURI, eventID, payload, assertion)
		if DeletionOutcomeIsTerminal(outcome) {
			result.Status = outcome
			return result
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

func (s *Service) sendDeletionCallback(ctx context.Context, callback, eventID string, payload []byte, assertion string) (string, string) {
	requestCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, callback, bytes.NewReader(payload))
	if err != nil {
		return "unresolved", "invalid_callback"
	}
	request.Header.Set("Authorization", "Bearer "+assertion)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-MW-Identity-Event-ID", eventID)
	request.Header.Set("User-Agent", "mw-identity-account-lifecycle/1.0")
	response, err := s.httpClient.Do(request)
	if err != nil {
		if response != nil && response.StatusCode >= 300 && response.StatusCode < 400 {
			return "unresolved", "callback_redirect"
		}
		return "pending", "callback_unavailable"
	}
	return readDeletionResponse(response)
}
