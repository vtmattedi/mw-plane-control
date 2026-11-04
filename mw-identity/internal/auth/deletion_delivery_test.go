package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type lifecycleTestSigner struct{ claims map[string]any }

func (s lifecycleTestSigner) SignJWT(_ context.Context, claims map[string]any) (string, error) {
	for key, value := range claims {
		s.claims[key] = value
	}
	return "signed.test.assertion", nil
}

func TestDeletionDeliveryMakesOneAttemptAndBindsOriginalEvent(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(map[bool]string{false: "retryable", true: "terminal"}[complete], func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("X-MW-Identity-Event-ID") != "event" {
					t.Error("event binding missing")
				}
				if complete {
					w.Write([]byte(`{"version":1,"outcome":"confirmed_cleanup"}`))
				} else {
					w.WriteHeader(503)
				}
			}))
			defer server.Close()
			claims := map[string]any{}
			s := &Service{httpClient: server.Client(), lifecycleSigner: lifecycleTestSigner{claims}, publicIssuer: "https://identity.example"}
			now := time.Now().UTC()
			status, _ := s.deliverDurableDeletion(context.Background(), DurableDelivery{EventID: "event", OperationID: "operation", Subject: "subject", ApplicationID: "application", ClientID: "client", CallbackURI: server.URL, RequestedAt: now.Add(-time.Hour)}, now)
			want := "pending"
			if complete {
				want = "confirmed_cleanup"
			}
			if calls != 1 || status != want {
				t.Fatalf("one delivery = %s, requests=%d", status, calls)
			}
			if claims["reason"] != DeletionReasonAccountDeleted || claims["operation_id"] != "operation" || claims["jti"] != "event" || claims["requested_at"] != now.Add(-time.Hour).Format(time.RFC3339) {
				t.Fatal("signed deletion bindings lost")
			}
		})
	}
}
