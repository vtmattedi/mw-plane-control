package recovery

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"filippo.io/age"
	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/webauthn"
	"os"
	"path/filepath"
	"testing"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitywebauthn"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	storesqlite "github.com/vtmattedi/mw-plane-control/mw-identity/internal/store/sqlite"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
)

const testPassphrase = "correct horse backup recovery phrase"

func fixture(t *testing.T) config.Config {
	t.Helper()
	key := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	cfg := config.Config{DatabasePath: filepath.Join(t.TempDir(), "source.db"), PublicIssuer: "https://identity.example.test", WebAuthnRPID: "identity.example.test", WebAuthnOrigins: []string{"https://identity.example.test"}, SigningKeys: "1:" + key, SigningActiveKeyVersion: 1, WebAuthnCredentialKeys: "1:" + key, WebAuthnCredentialActiveKeyVersion: 1, TOTPSecretKeys: "1:" + key, TOTPSecretActiveKeyVersion: 1}
	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := storesqlite.ApplyMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	keyring, err := signing.ParseKeyring(cfg.SigningKeys, 1)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := signing.NewManager(storesqlite.NewStore(db), keyring)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.EnsureActiveKey(ctx); err != nil {
		t.Fatal(err)
	}
	queries := []string{
		`INSERT INTO identity_users(id,email,email_normalized,display_name,password_hash,is_superadmin,created_at_unix,updated_at_unix) VALUES ('root-id','root@example.test','root@example.test','Root','original-password-hash',1,1,1)`,
		`INSERT INTO identity_users(id,email,email_normalized,display_name,password_hash,recovery_pending,created_at_unix,updated_at_unix) VALUES ('user-id','user@example.test','user@example.test','User','another-hash',1,1,1)`,
		`INSERT INTO identity_sessions(id,user_id,token_hash,csrf_hash,expires_at_unix,created_at_unix,last_seen_at_unix) VALUES ('session','root-id','token','csrf',9999999999,1,1)`,
		`INSERT INTO identity_recovery_requests(id,user_id,token_hash,status,expires_at_unix,created_at_unix) VALUES ('recover','user-id','recovery-token','pending',9999999999,1)`,
		`INSERT INTO identity_applications(id,name,client_id,application_type,created_at_unix,updated_at_unix) VALUES ('app-id','Admin','original-client','confidential',1,1)`,
		`INSERT INTO identity_application_assignments(application_id,user_id,role,created_at_unix,updated_at_unix) VALUES ('app-id','user-id','superadmin',1,1)`,
		`INSERT INTO identity_webauthn_users(user_id,rpid,user_handle) VALUES ('root-id','identity.example.test',X'010203')`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	webKeys, err := identitywebauthn.NewKeyring(cfg.WebAuthnCredentialKeys, 1)
	if err != nil {
		t.Fatal(err)
	}
	id := []byte("credential-id")
	aad := []byte("mw-plane-control:identity-webauthn:credential:v1:root-id:" + cfg.WebAuthnRPID + ":" + base64.RawURLEncoding.EncodeToString(id))
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cose, err := webauthncbor.Marshal(map[int]any{1: 1, 3: -8, -1: 6, -2: []byte(public)})
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(webauthn.Credential{ID: id, PublicKey: cose})
	if err != nil {
		t.Fatal(err)
	}
	sealed, nonce, v, err := webKeys.Encrypt(serialized, aad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO identity_webauthn_credentials(user_id,rpid,credential_id,credential_ciphertext,credential_nonce,credential_key_version) VALUES (?,?,?,?,?,?)`, "root-id", cfg.WebAuthnRPID, id, sealed, nonce, v); err != nil {
		t.Fatal(err)
	}
	totpKeys, err := identitystrong.NewKeyring(cfg.TOTPSecretKeys, 1)
	if err != nil {
		t.Fatal(err)
	}
	sealed, nonce, v, err = totpKeys.Encrypt([]byte("totp-security-material"), []byte("mw-identity-totp:root-id"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO identity_totp_credentials(user_id,secret_ciphertext,secret_nonce,secret_key_version,enabled,created_at_unix,updated_at_unix) VALUES (?,?,?,?,1,1,1)`, "root-id", sealed, nonce, v); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestEncryptedBackupAndPreparePreserveRootAndInvalidateSession(t *testing.T) {
	cfg := fixture(t)
	ctx := context.Background()
	dir := t.TempDir()
	archive := filepath.Join(dir, "backup.age")
	created, err := Create(ctx, cfg, archive, testPassphrase)
	if err != nil {
		t.Fatal(err)
	}
	if created.Summary.Users != 2 || created.Summary.UsableGlobalSuperadmins != 1 {
		t.Fatalf("unexpected summary: %+v", created.Summary)
	}
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("root@example.test")) || bytes.Contains(raw, []byte("original-password-hash")) {
		t.Fatal("plaintext security data in archive")
	}
	if _, err := Validate(ctx, cfg, archive, testPassphrase); err != nil {
		t.Fatal(err)
	}
	prepared := filepath.Join(dir, "prepared")
	if _, err := Prepare(ctx, cfg, archive, testPassphrase, prepared); err != nil {
		t.Fatal(err)
	}
	db, err := openReadOnly(filepath.Join(prepared, "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var hash string
	var root, count, pending int
	var status string
	if err := db.QueryRow(`SELECT password_hash,is_superadmin FROM identity_users WHERE id='root-id'`).Scan(&hash, &root); err != nil {
		t.Fatal(err)
	}
	if hash != "original-password-hash" || root != 1 {
		t.Fatal("root identity changed")
	}
	var role string
	if err := db.QueryRow(`SELECT role FROM identity_application_assignments WHERE application_id='app-id' AND user_id='user-id'`).Scan(&role); err != nil || role != "superadmin" {
		t.Fatal("application assignment changed")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM identity_sessions`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("old sessions survive: %d %v", count, err)
	}
	if err := db.QueryRow(`SELECT recovery_pending FROM identity_users WHERE id='user-id'`).Scan(&pending); err != nil || pending != 1 {
		t.Fatal("recovery restriction was cleared")
	}
	if err := db.QueryRow(`SELECT status FROM identity_recovery_requests WHERE id='recover'`).Scan(&status); err != nil || status != "cancelled" {
		t.Fatal("old recovery capability survives")
	}
	source, err := openReadOnly(cfg.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := source.QueryRow(`SELECT COUNT(*) FROM identity_sessions`).Scan(&count); err != nil || count != 1 {
		t.Fatal("live database was modified")
	}
	if _, err := Create(ctx, cfg, archive, testPassphrase); err == nil {
		t.Fatal("existing archive overwritten")
	}
}

func TestValidationRejectsWrongEnvironmentAndTampering(t *testing.T) {
	cfg := fixture(t)
	archive := filepath.Join(t.TempDir(), "backup.age")
	ctx := context.Background()
	if _, err := Create(ctx, cfg, archive, testPassphrase); err != nil {
		t.Fatal(err)
	}
	bad := cfg
	bad.PublicIssuer = "https://other.test"
	if _, err := Validate(ctx, bad, archive, testPassphrase); err == nil {
		t.Fatal("wrong issuer accepted")
	}
	bad = cfg
	bad.SigningKeys = "1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))
	if _, err := Validate(ctx, bad, archive, testPassphrase); err == nil {
		t.Fatal("wrong encryption keys accepted")
	}
	bad = cfg
	bad.WebAuthnCredentialKeys = "1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))
	if _, err := Validate(ctx, bad, archive, testPassphrase); err == nil {
		t.Fatal("wrong WebAuthn keys accepted")
	}
	bad = cfg
	bad.TOTPSecretKeys = "1:" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))
	if _, err := Validate(ctx, bad, archive, testPassphrase); err == nil {
		t.Fatal("wrong TOTP keys accepted")
	}
	if _, err := Validate(ctx, cfg, archive, "incorrect recovery passphrase"); err == nil {
		t.Fatal("wrong passphrase accepted")
	}
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 1
	if err := os.WriteFile(archive, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "prepared")
	if _, err := Prepare(ctx, cfg, archive, testPassphrase, out); err == nil {
		t.Fatal("tampered archive accepted")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("failed preparation left plaintext output")
	}
}

func TestRejectUnsafeAndOversizedArchiveEntries(t *testing.T) {
	cfg := fixture(t)
	for _, h := range []tar.Header{
		{Name: "../escape.db", Typeflag: tar.TypeReg, Size: 1},
		{Name: "identity.db", Typeflag: tar.TypeSymlink, Linkname: "../source.db"},
		{Name: "identity.db", Typeflag: tar.TypeReg, Size: MaxDatabaseBytes + 1},
	} {
		t.Run(h.Name+string(h.Typeflag), func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "bad.age")
			f, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			recipient, err := age.NewScryptRecipient(testPassphrase)
			if err != nil {
				t.Fatal(err)
			}
			recipient.SetWorkFactor(10)
			encrypted, err := age.Encrypt(f, recipient)
			if err != nil {
				t.Fatal(err)
			}
			tw := tar.NewWriter(encrypted)
			if err := tw.WriteHeader(&h); err != nil {
				t.Fatal(err)
			}
			// A deliberately incomplete payload still has a valid age envelope.
			if err := encrypted.Close(); err != nil {
				t.Fatal(err)
			}
			f.Close()
			if _, err := Validate(context.Background(), cfg, archive, testPassphrase); err == nil {
				t.Fatal("unsafe entry accepted")
			}
		})
	}
}

func TestInspectRejectsFutureSchema(t *testing.T) {
	cfg := fixture(t)
	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations(name,applied_at) VALUES ('999_future.sql','now')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Inspect(context.Background(), cfg.DatabasePath, cfg); err == nil {
		t.Fatal("unsupported schema accepted")
	}
}

func TestBackupAllowsRootRecoveryButNormalPrepareBlocksMissingRoot(t *testing.T) {
	cfg := fixture(t)
	db, err := storesqlite.Open(cfg.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE identity_users SET enabled=0 WHERE is_superadmin=1`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	archive := filepath.Join(t.TempDir(), "backup.age")
	ctx := context.Background()
	if _, err := Create(ctx, cfg, archive, testPassphrase); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(ctx, cfg, archive, testPassphrase); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(ctx, cfg, archive, testPassphrase, filepath.Join(t.TempDir(), "prepared")); err == nil {
		t.Fatal("normal restore permits root lockout")
	}
}

func TestCreateRejectsMissingSourceWithoutCreatingDatabase(t *testing.T) {
	cfg := fixture(t)
	cfg.DatabasePath = filepath.Join(t.TempDir(), "missing.db")
	if _, err := Create(context.Background(), cfg, filepath.Join(t.TempDir(), "backup.age"), testPassphrase); err == nil {
		t.Fatal("missing database accepted")
	}
	if _, err := os.Stat(cfg.DatabasePath); !os.IsNotExist(err) {
		t.Fatal("missing source was created")
	}
}
