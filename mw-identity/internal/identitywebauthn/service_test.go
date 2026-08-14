package identitywebauthn

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
)

type policyStore struct {
	user       UserRecord
	ceremonies []CeremonyRecord
}

func (s *policyStore) IdentityUserByID(context.Context, string) (auth.User, error) {
	return auth.User{ID: "usr_test", Email: "test@example.com", DisplayName: "Test", Enabled: true}, nil
}
func (s *policyStore) EnsureIdentityWebAuthnUser(_ context.Context, userID, rpid string, handle []byte) (UserRecord, error) {
	if s.user.UserID == "" {
		s.user = UserRecord{ID: 1, UserID: userID, RPID: rpid, UserHandle: append([]byte(nil), handle...), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	return s.user, nil
}
func (s *policyStore) IdentityWebAuthnUser(context.Context, string, string) (UserRecord, error) {
	if s.user.UserID == "" {
		return UserRecord{}, ErrNotFound
	}
	return s.user, nil
}
func (s *policyStore) FindIdentityWebAuthnUserForCredential(context.Context, string, []byte, []byte) (auth.User, UserRecord, error) {
	return auth.User{}, UserRecord{}, ErrNotFound
}
func (s *policyStore) ListIdentityWebAuthnCredentials(context.Context, string, string) ([]CredentialRecord, error) {
	return nil, nil
}
func (s *policyStore) InsertIdentityWebAuthnCredential(context.Context, CredentialRecord) (CredentialRecord, error) {
	return CredentialRecord{}, nil
}
func (s *policyStore) UpdateIdentityWebAuthnCredential(context.Context, CredentialRecord, string) error {
	return nil
}
func (s *policyStore) RevokeIdentityWebAuthnCredential(context.Context, string, string, []byte, string) error {
	return nil
}
func (s *policyStore) RevokeOtherIdentityWebAuthnCredentials(context.Context, string, string, []byte, string) error {
	return nil
}
func (s *policyStore) CreateIdentityWebAuthnCeremony(_ context.Context, r CeremonyRecord) error {
	s.ceremonies = append(s.ceremonies, r)
	return nil
}
func (s *policyStore) ConsumeIdentityWebAuthnCeremony(context.Context, string, int64) (CeremonyRecord, error) {
	return CeremonyRecord{}, ErrNotFound
}
func (s *policyStore) DeleteExpiredIdentityWebAuthnCeremonies(context.Context, int64) error {
	return nil
}
func (s *policyStore) RecordAuditEvent(context.Context, audit.Event) error { return nil }

func TestBeginRegistrationAndDiscoverableLoginPolicy(t *testing.T) {
	store := &policyStore{}
	key := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x55}, 32))
	service, err := NewService(store, Options{
		RPID:                       "example.com",
		RPName:                     "MW Identity Test",
		Origins:                    []string{"https://example.com"},
		CredentialKeys:             "1:" + key,
		CredentialActiveKeyVersion: 1,
		CeremonyTTL:                5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	user := auth.User{ID: "usr_test", Email: "test@example.com", DisplayName: "Test", Enabled: true}
	creation, token, err := service.BeginRegistration(t.Context(), user, "Windows Hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "mwwa_") {
		t.Fatalf("token = %q", token)
	}

	body, _ := json.Marshal(creation)
	text := string(body)
	for _, expected := range []string{`"residentKey":"required"`, `"userVerification":"required"`, `"attestation":"none"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("registration missing %s: %s", expected, text)
		}
	}

	assertion, loginToken, err := service.BeginLogin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(loginToken, "mwwa_") {
		t.Fatalf("login token = %q", loginToken)
	}
	body, _ = json.Marshal(assertion)
	if !strings.Contains(string(body), `"userVerification":"required"`) {
		t.Fatalf("login does not require UV: %s", body)
	}

	if len(store.ceremonies) != 2 {
		t.Fatalf("ceremonies = %d", len(store.ceremonies))
	}
	if store.ceremonies[0].TokenHash == token {
		t.Fatal("raw ceremony token stored")
	}
}
