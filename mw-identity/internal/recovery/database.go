package recovery

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/go-webauthn/webauthn/webauthn"
	"io/fs"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitystrong"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/identitywebauthn"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/signing"
	"github.com/vtmattedi/mw-plane-control/mw-identity/migrations"
	_ "modernc.org/sqlite"
)

type Summary struct {
	Users                   int              `json:"users"`
	GlobalSuperadmins       int              `json:"global_superadmins"`
	UsableGlobalSuperadmins int              `json:"usable_global_superadmins"`
	Applications            int              `json:"applications"`
	Assignments             int              `json:"assignments"`
	RecoveryPendingUsers    int              `json:"recovery_pending_users"`
	Schema                  []string         `json:"schema"`
	RequiredKeyVersions     map[string][]int `json:"required_key_versions"`
}

func sqliteLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func databaseURI(path, mode string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	slash := filepath.ToSlash(absolute)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	u := url.URL{Scheme: "file", Path: slash, RawQuery: "mode=" + mode}
	return u.String(), nil
}
func openReadOnly(path string) (*sql.DB, error) {
	if err := regularFile(path); err != nil {
		return nil, err
	}
	uri, err := databaseURI(path, "ro")
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Inspect validates every persistent encrypted credential with the configured keyrings.
// Exact schema matching is intentional for v1: no automatic migration during recovery.
func Inspect(ctx context.Context, path string, cfg config.Config) (Summary, error) {
	s := Summary{RequiredKeyVersions: map[string][]int{}}
	db, err := openReadOnly(path)
	if err != nil {
		return s, err
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return s, err
	}
	if integrity != "ok" {
		return s, fmt.Errorf("database integrity check failed")
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return s, err
	}
	violated := rows.Next()
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return s, rowErr
	}
	if violated {
		return s, fmt.Errorf("database foreign key check failed")
	}
	rows, err = db.QueryContext(ctx, "SELECT name FROM schema_migrations ORDER BY name")
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return s, err
		}
		s.Schema = append(s.Schema, name)
	}
	rowErr = rows.Err()
	rows.Close()
	if rowErr != nil {
		return s, rowErr
	}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return s, err
	}
	var expected []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			expected = append(expected, e.Name())
		}
	}
	sort.Strings(expected)
	if !reflect.DeepEqual(s.Schema, expected) {
		return s, fmt.Errorf("backup schema does not match this recovery binary")
	}
	counts := []struct {
		query string
		dst   *int
	}{
		{"SELECT COUNT(*) FROM identity_users", &s.Users},
		{"SELECT COUNT(*) FROM identity_users WHERE is_superadmin=1", &s.GlobalSuperadmins},
		{"SELECT COUNT(*) FROM identity_applications", &s.Applications},
		{"SELECT COUNT(*) FROM identity_application_assignments", &s.Assignments},
		{"SELECT COUNT(*) FROM identity_users WHERE recovery_pending=1", &s.RecoveryPendingUsers},
		{`SELECT COUNT(*) FROM identity_users u WHERE u.is_superadmin=1 AND u.enabled=1 AND u.recovery_pending=0 AND EXISTS (SELECT 1 FROM identity_webauthn_credentials c JOIN identity_webauthn_users w ON w.user_id=c.user_id AND w.rpid=c.rpid WHERE c.user_id=u.id AND c.revoked_at IS NULL AND c.rpid=? AND length(w.user_handle)>0)`, &s.UsableGlobalSuperadmins},
	}
	for _, c := range counts {
		args := []any{}
		if c.dst == &s.UsableGlobalSuperadmins {
			args = append(args, cfg.WebAuthnRPID)
		}
		if err := db.QueryRowContext(ctx, c.query, args...).Scan(c.dst); err != nil {
			return s, err
		}
	}
	if err := inspectSigning(ctx, db, cfg, &s); err != nil {
		return s, err
	}
	if err := inspectCredentials(ctx, db, cfg, &s); err != nil {
		return s, err
	}
	for key := range s.RequiredKeyVersions {
		sort.Ints(s.RequiredKeyVersions[key])
	}
	return s, nil
}

func addVersion(s *Summary, name string, v int) {
	for _, old := range s.RequiredKeyVersions[name] {
		if old == v {
			return
		}
	}
	s.RequiredKeyVersions[name] = append(s.RequiredKeyVersions[name], v)
}
func inspectSigning(ctx context.Context, db *sql.DB, cfg config.Config, s *Summary) error {
	keyring, err := signing.ParseKeyring(cfg.SigningKeys, cfg.SigningActiveKeyVersion)
	if err != nil {
		return fmt.Errorf("invalid signing keyring")
	}
	rows, err := db.QueryContext(ctx, "SELECT kid,status,private_key_ciphertext,private_key_nonce,encryption_key_version FROM identity_signing_keys")
	if err != nil {
		return err
	}
	defer rows.Close()
	active := 0
	for rows.Next() {
		var kid, status string
		var sealed, nonce []byte
		var v int
		if err := rows.Scan(&kid, &status, &sealed, &nonce, &v); err != nil {
			return err
		}
		key, ok := keyring.Key(v)
		if !ok {
			return fmt.Errorf("signing key version %d is missing", v)
		}
		block, err := aes.NewCipher(key)
		clear(key)
		if err != nil {
			return err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return err
		}
		if len(nonce) != gcm.NonceSize() {
			return fmt.Errorf("invalid signing nonce")
		}
		plain, err := gcm.Open(nil, nonce, sealed, []byte("mw-identity-signing-key:"+kid))
		if err != nil {
			return fmt.Errorf("signing record cannot decrypt with supplied environment")
		}
		parsed, parseErr := x509.ParsePKCS8PrivateKey(plain)
		clear(plain)
		if parseErr != nil {
			return fmt.Errorf("invalid restored signing key")
		}
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok || rsaKey.Validate() != nil {
			return fmt.Errorf("invalid restored RSA signing key")
		}
		if status == "active" {
			active++
		}
		addVersion(s, "IDENTITY_SIGNING_KEYS", v)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if active != 1 {
		return fmt.Errorf("backup must contain one usable active signing key")
	}
	return nil
}

func inspectCredentials(ctx context.Context, db *sql.DB, cfg config.Config, s *Summary) error {
	rows, err := db.QueryContext(ctx, "SELECT user_id,rpid,credential_id,credential_ciphertext,credential_nonce,credential_key_version FROM identity_webauthn_credentials")
	if err != nil {
		return err
	}
	var webKeys *identitywebauthn.Keyring
	for rows.Next() {
		if webKeys == nil {
			webKeys, err = identitywebauthn.NewKeyring(cfg.WebAuthnCredentialKeys, cfg.WebAuthnCredentialActiveKeyVersion)
			if err != nil {
				rows.Close()
				return fmt.Errorf("invalid WebAuthn keyring")
			}
		}
		var user, rpid, sealed, nonce string
		var id []byte
		var v int
		if err := rows.Scan(&user, &rpid, &id, &sealed, &nonce, &v); err != nil {
			rows.Close()
			return err
		}
		aad := []byte(fmt.Sprintf("mw-plane-control:identity-webauthn:credential:v1:%s:%s:%s", user, rpid, base64.RawURLEncoding.EncodeToString(id)))
		plain, err := webKeys.Decrypt(sealed, nonce, v, aad)
		if err != nil {
			rows.Close()
			return fmt.Errorf("WebAuthn credential cannot decrypt; required version %d", v)
		}
		var credential webauthn.Credential
		decodeErr := json.Unmarshal(plain, &credential)
		clear(plain)
		if decodeErr != nil || !bytes.Equal(credential.ID, id) || len(credential.PublicKey) == 0 {
			rows.Close()
			return fmt.Errorf("invalid serialized WebAuthn credential")
		}
		if _, err := webauthncose.ParsePublicKey(credential.PublicKey); err != nil {
			rows.Close()
			return fmt.Errorf("invalid restored WebAuthn public key")
		}
		addVersion(s, "IDENTITY_WEBAUTHN_CREDENTIAL_KEYS", v)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return rowErr
	}
	rows, err = db.QueryContext(ctx, "SELECT user_id,secret_ciphertext,secret_nonce,secret_key_version FROM identity_totp_credentials")
	if err != nil {
		return err
	}
	defer rows.Close()
	var totpKeys *identitystrong.Keyring
	for rows.Next() {
		if totpKeys == nil {
			totpKeys, err = identitystrong.NewKeyring(cfg.TOTPSecretKeys, cfg.TOTPSecretActiveKeyVersion)
			if err != nil {
				return fmt.Errorf("invalid TOTP keyring")
			}
		}
		var user, sealed, nonce string
		var v int
		if err := rows.Scan(&user, &sealed, &nonce, &v); err != nil {
			return err
		}
		plain, err := totpKeys.Decrypt(sealed, nonce, v, []byte("mw-identity-totp:"+user))
		if err != nil {
			return fmt.Errorf("TOTP credential cannot decrypt; required version %d", v)
		}
		clear(plain)
		addVersion(s, "IDENTITY_TOTP_SECRET_KEYS", v)
	}
	return rows.Err()
}

func invalidate(ctx context.Context, path string) error {
	uri, err := databaseURI(path, "rw")
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, query := range []string{
		"DELETE FROM identity_access_tokens", "DELETE FROM identity_authorization_codes", "DELETE FROM identity_sessions", "DELETE FROM identity_mfa_challenges", "DELETE FROM identity_webauthn_ceremonies",
		"UPDATE identity_recovery_requests SET status='cancelled' WHERE status='pending'",
	} {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return tx.Commit()
}
