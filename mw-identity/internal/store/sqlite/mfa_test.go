package sqlite

import (
	"context"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"testing"
	"time"
)

func TestMFABudgetExpiryAndConcurrentConsumption(t *testing.T) {
	f := newRemovalFixture(t, "")
	ctx := context.Background()
	c := auth.MFAChallenge{TokenHash: "challenge-hash", BindingHash: "browser-hash", UserID: "usr_remove", PasswordFingerprint: "fingerprint", ExpiresAt: f.now.Add(time.Minute)}
	if err := f.store.CreateMFAChallenge(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AttemptMFAChallenge(ctx, c.TokenHash, "wrong-browser", f.now); err == nil {
		t.Fatal("wrong browser accepted")
	}
	for range 5 {
		if _, err := f.store.AttemptMFAChallenge(ctx, c.TokenHash, c.BindingHash, f.now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.store.AttemptMFAChallenge(ctx, c.TokenHash, c.BindingHash, f.now); err == nil {
		t.Fatal("budget reset")
	}
	c.TokenHash = "expired"
	c.ExpiresAt = f.now
	if err := f.store.CreateMFAChallenge(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AttemptMFAChallenge(ctx, c.TokenHash, c.BindingHash, f.now); err == nil {
		t.Fatal("expired challenge accepted")
	}
	c.TokenHash = "race"
	c.ExpiresAt = f.now.Add(time.Minute)
	if err := f.store.CreateMFAChallenge(ctx, c); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- f.store.CompleteMFAChallenge(ctx, c.TokenHash, c.BindingHash, f.now) }()
	}
	successes := 0
	for range 2 {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("consumed %d times", successes)
	}
}
