package main

import (
	"errors"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		SessionTTL:     time.Hour,
		TransactionTTL: 5 * time.Minute,
		PendingTTL:     5 * time.Minute,
		Users: []LocalCredential{
			{ID: "u1", Email: "same@example.test", Password: "local-one"},
			{ID: "u2", Email: "other@example.test", Password: "local-two"},
		},
	}
}

func TestDuplicateSubjectCannotMoveBetweenLocalAccounts(t *testing.T) {
	s := NewState(testConfig())
	identity := IdentityClaims{Issuer: "https://identity.example", Subject: "usr_abc", Email: "same@example.test"}
	if _, err := s.LinkIdentity("u1", identity); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkIdentity("u2", identity); !errors.Is(err, ErrDuplicateSubject) {
		t.Fatalf("error = %v; want duplicate subject", err)
	}
	linked, ok := s.Identity(identity.Issuer, identity.Subject)
	if !ok || linked.LocalUserID != "u1" {
		t.Fatalf("mapping moved: %+v", linked)
	}
}

func TestEmailMatchDoesNotCreateLink(t *testing.T) {
	s := NewState(testConfig())
	identity := IdentityClaims{Issuer: "https://identity.example", Subject: "usr_abc", Email: "same@example.test"}
	if _, ok := s.Identity(identity.Issuer, identity.Subject); ok {
		t.Fatal("identity unexpectedly linked")
	}
	pendingRaw, err := s.CreatePending(identity)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.Pending(pendingRaw)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Identity.Email != "same@example.test" {
		t.Fatal("pending identity email changed")
	}
	if _, ok := s.Identity(identity.Issuer, identity.Subject); ok {
		t.Fatal("email equality auto-linked account")
	}
}

func TestPendingRequiresExplicitLocalAccountChoice(t *testing.T) {
	s := NewState(testConfig())
	identity := IdentityClaims{Issuer: "https://identity.example", Subject: "usr_abc", Email: "same@example.test"}
	raw, _ := s.CreatePending(identity)
	linked, err := s.CompletePending(raw, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if linked.LocalUserID != "u1" {
		t.Fatalf("linked user = %q", linked.LocalUserID)
	}
	if _, err := s.Pending(raw); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending token reusable: %v", err)
	}
}

func TestOIDCTransactionIsSingleUse(t *testing.T) {
	s := NewState(testConfig())
	_, rawState, _, err := s.BeginTransaction("signin", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimTransaction(rawState); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimTransaction(rawState); !errors.Is(err, ErrNotFound) {
		t.Fatalf("transaction reusable: %v", err)
	}
}

func TestUnlinkRemovesIssuerSubjectMapping(t *testing.T) {
	s := NewState(testConfig())
	identity := IdentityClaims{Issuer: "https://identity.example", Subject: "usr_abc"}
	if _, err := s.LinkIdentity("u1", identity); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UnlinkIdentity("u1", identity.Issuer); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Identity(identity.Issuer, identity.Subject); ok {
		t.Fatal("mapping survived unlink")
	}
}
