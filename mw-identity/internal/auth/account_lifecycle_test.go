package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type lifecycleTestSigner struct{}

func (lifecycleTestSigner) SignJWT(context.Context, map[string]any) (string, error) {
	return "signed.lifecycle.assertion", nil
}

func TestDeletionCallbackUsesThreeTotalAttempts(t *testing.T) {
	original := deletionRetryDelays
	deletionRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { deletionRetryDelays = original })

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		if r.Header.Get("X-MW-Identity-Event-ID") == "" {
			t.Error("missing lifecycle event id")
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	service := &Service{httpClient: server.Client(), lifecycleSigner: lifecycleTestSigner{}, publicIssuer: "https://identity.example"}
	result := service.deliverDeletion(context.Background(), "usr_public", ConnectedApplication{
		ID: "app_woi", ClientID: "woi", DeleteCallbackURI: server.URL,
	}, time.Unix(1_700_000_000, 0).UTC(), DeletionReasonAccountDeleted)

	if result.Status != "failed" || result.Attempts != 3 || attempts.Load() != 3 {
		t.Fatalf("unexpected delivery result: %#v, requests=%d", result, attempts.Load())
	}
}

func TestDeletionCallbackStopsAfterSuccess(t *testing.T) {
	original := deletionRetryDelays
	deletionRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { deletionRetryDelays = original })

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := &Service{httpClient: server.Client(), lifecycleSigner: lifecycleTestSigner{}, publicIssuer: "https://identity.example"}
	result := service.deliverDeletion(context.Background(), "usr_public", ConnectedApplication{
		ID: "app_woi", ClientID: "woi", DeleteCallbackURI: server.URL,
	}, time.Now(), DeletionReasonAccountDeleted)

	if result.Status != "delivered" || result.Attempts != 2 || attempts.Load() != 2 {
		t.Fatalf("unexpected delivery result: %#v, requests=%d", result, attempts.Load())
	}
}

func TestNormalizeUsername(t *testing.T) {
	value, err := NormalizeUsername("  Public.User_1 ")
	if err != nil || value != "public.user_1" {
		t.Fatalf("NormalizeUsername() = %q, %v", value, err)
	}
	if _, err := NormalizeUsername("no spaces allowed"); err == nil {
		t.Fatal("expected invalid username error")
	}
}
