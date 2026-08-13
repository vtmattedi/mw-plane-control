package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/audit"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/auth"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

func testIdentityStore(t *testing.T) *Store {
	t.Helper()

	db, err := Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := ApplyMigrations(
		context.Background(),
		db,
		migrations.FS,
	); err != nil {
		t.Fatal(err)
	}

	return NewStore(db)
}

func TestIdentitySessionStoresHashesOnly(t *testing.T) {
	store := testIdentityStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	passwordHash, err := auth.HashPassword(
		[]byte("correct horse battery staple"),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = store.CreateIdentityUser(ctx, auth.NewUser{
		ID:              "usr_test",
		Email:           "admin@example.com",
		EmailNormalized: "admin@example.com",
		DisplayName:     "Admin",
		PasswordHash:    passwordHash,
		Enabled:         true,
		IsSuperadmin:    true,
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatal(err)
	}

	rawSession := "mwid_raw-session-must-not-be-stored"
	rawCSRF := "mwcsrf_raw-csrf-must-not-be-stored"

	err = store.CreateIdentitySession(
		ctx,
		auth.SessionRecord{
			ID:         "sid_test",
			UserID:     "usr_test",
			TokenHash:  auth.HashToken(rawSession),
			CSRFHash:   auth.HashToken(rawCSRF),
			ExpiresAt:  now.Add(time.Hour),
			CreatedAt:  now,
			LastSeenAt: now,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	var tokenHash, csrfHash string
	err = store.db.QueryRow(
		`SELECT token_hash, csrf_hash
		 FROM identity_sessions
		 WHERE id = 'sid_test'`,
	).Scan(&tokenHash, &csrfHash)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(tokenHash, rawSession) ||
		strings.Contains(csrfHash, rawCSRF) {
		t.Fatal("raw session or CSRF token was persisted")
	}

	if !auth.TokenMatchesHash(rawSession, tokenHash) {
		t.Fatal("stored session hash does not match raw token")
	}
	if !auth.TokenMatchesHash(rawCSRF, csrfHash) {
		t.Fatal("stored csrf hash does not match raw token")
	}
}

func TestExpiredSessionIsNotReturned(t *testing.T) {
	store := testIdentityStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	hash, err := auth.HashPassword(
		[]byte("correct horse battery staple"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.CreateIdentityUser(ctx, auth.NewUser{
		ID:              "usr_expired",
		Email:           "expired@example.com",
		EmailNormalized: "expired@example.com",
		DisplayName:     "Expired",
		PasswordHash:    hash,
		Enabled:         true,
		CreatedAt:       now,
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.CreateIdentitySession(
		ctx,
		auth.SessionRecord{
			ID:         "sid_expired",
			UserID:     "usr_expired",
			TokenHash:  auth.HashToken("raw"),
			CSRFHash:   auth.HashToken("csrf"),
			ExpiresAt:  now.Add(-time.Minute),
			CreatedAt:  now.Add(-time.Hour),
			LastSeenAt: now.Add(-time.Hour),
		},
	); err != nil {
		t.Fatal(err)
	}

	_, err = store.IdentitySessionByTokenHash(
		ctx,
		auth.HashToken("raw"),
		now,
	)
	if err != auth.ErrNotFound {
		t.Fatalf("error = %v; want ErrNotFound", err)
	}
}

func TestAuditWrite(t *testing.T) {
	store := testIdentityStore(t)

	err := store.RecordAuditEvent(
		context.Background(),
		audit.Event{
			ID:        "aud_test",
			EventType: "identity.test",
			Status:    "success",
			Severity:  "info",
			CreatedAt: time.Now().UTC(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}
