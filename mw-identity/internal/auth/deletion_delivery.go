package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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
