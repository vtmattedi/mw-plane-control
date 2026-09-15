// Package recovery implements encrypted Identity snapshots and isolated restore preparation.
// It deliberately does not replace a running service's database.
package recovery

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"filippo.io/age"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/buildinfo"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
)

const Format = "mw-identity-recovery-v1"
const MaxDatabaseBytes int64 = 2 << 30
const maxManifestBytes int64 = 1 << 20

type Environment struct {
	Issuer  string   `json:"issuer"`
	RPID    string   `json:"webauthn_rp_id"`
	Origins []string `json:"webauthn_origins"`
}
type Manifest struct {
	Format        string         `json:"format"`
	CreatedAt     time.Time      `json:"created_at_utc"`
	Build         buildinfo.Info `json:"build"`
	Coverage      []string       `json:"coverage"`
	Environment   Environment    `json:"environment"`
	DatabaseBytes int64          `json:"database_bytes"`
	SHA256        string         `json:"sha256"`
	Summary       Summary        `json:"summary"`
}

func environment(cfg config.Config) Environment {
	return Environment{cfg.PublicIssuer, cfg.WebAuthnRPID, cfg.WebAuthnOrigins}
}

// Create snapshots an existing database without migrating or modifying its schema.
func Create(ctx context.Context, cfg config.Config, output, passphrase string) (Manifest, error) {
	var m Manifest
	if len(passphrase) < 16 {
		return m, fmt.Errorf("use a recovery passphrase of at least 16 bytes")
	}
	if err := regularFile(cfg.DatabasePath); err != nil {
		return m, err
	}
	stage, err := os.MkdirTemp("", "mw-identity-backup-*")
	if err != nil {
		return m, err
	}
	defer os.RemoveAll(stage)
	snapshot := filepath.Join(stage, "identity.db")
	db, err := openReadOnly(cfg.DatabasePath)
	if err != nil {
		return m, err
	}
	// VACUUM INTO is SQLite's consistent online snapshot. The source is read-only.
	escaped := sqliteLiteral(snapshot)
	_, err = db.ExecContext(ctx, "VACUUM INTO "+escaped)
	closeErr := db.Close()
	if err != nil {
		return m, fmt.Errorf("snapshot: %w", err)
	}
	if closeErr != nil {
		return m, closeErr
	}
	if err := os.Chmod(snapshot, 0600); err != nil {
		return m, err
	}
	summary, err := Inspect(ctx, snapshot, cfg)
	if err != nil {
		return m, err
	}
	size, digest, err := fileDigest(snapshot)
	if err != nil {
		return m, err
	}
	m = Manifest{Format: Format, CreatedAt: time.Now().UTC(), Build: buildinfo.Current(), Coverage: []string{"identity"}, Environment: environment(cfg), DatabaseBytes: size, SHA256: digest, Summary: summary}
	raw, err := json.Marshal(m)
	if err != nil {
		return m, err
	}
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return m, err
	}
	// O_EXCL prevents accidental overwrite; the partial archive is removed on every error.
	out, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return m, err
	}
	complete := false
	defer func() {
		out.Close()
		if !complete {
			os.Remove(output)
		}
	}()
	encrypted, err := age.Encrypt(out, recipient)
	if err != nil {
		return m, err
	}
	tw := tar.NewWriter(encrypted)
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(raw)), Typeflag: tar.TypeReg}); err != nil {
		return m, err
	}
	if _, err := tw.Write(raw); err != nil {
		return m, err
	}
	if err := tw.WriteHeader(&tar.Header{Name: "identity.db", Mode: 0600, Size: size, Typeflag: tar.TypeReg}); err != nil {
		return m, err
	}
	f, err := os.Open(snapshot)
	if err != nil {
		return m, err
	}
	_, copyErr := io.Copy(tw, f)
	f.Close()
	if copyErr != nil {
		return m, copyErr
	}
	if err := tw.Close(); err != nil {
		return m, err
	}
	if err := encrypted.Close(); err != nil {
		return m, err
	}
	if err := out.Sync(); err != nil {
		return m, err
	}
	if err := out.Close(); err != nil {
		return m, err
	}
	complete = true
	return m, nil
}

// Validate authenticates the complete encrypted payload, then inspects a temporary copy.
func Validate(ctx context.Context, cfg config.Config, archive, passphrase string) (Manifest, error) {
	stage, err := os.MkdirTemp("", "mw-identity-validate-*")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(stage)
	return extract(ctx, cfg, archive, passphrase, stage)
}

// Prepare writes an isolated database with transient authentication state invalidated.
// The supplied output directory must not exist. It never opens the live database.
func Prepare(ctx context.Context, cfg config.Config, archive, passphrase, outputDir string) (Manifest, error) {
	if err := os.Mkdir(outputDir, 0700); err != nil {
		return Manifest{}, err
	}
	complete := false
	defer func() {
		if !complete {
			os.RemoveAll(outputDir)
		}
	}()
	m, err := extract(ctx, cfg, archive, passphrase, outputDir)
	if err != nil {
		return m, err
	}
	if m.Summary.UsableGlobalSuperadmins == 0 {
		return m, fmt.Errorf("normal restore requires an enabled global superadmin with a passkey and no pending recovery; see the administrator recovery runbook")
	}
	path := filepath.Join(outputDir, "identity.db")
	if err := invalidate(ctx, path); err != nil {
		return m, err
	}
	if _, err := Inspect(ctx, path, cfg); err != nil {
		return m, err
	}
	complete = true
	return m, nil
}

func extract(ctx context.Context, cfg config.Config, archive, passphrase, stage string) (Manifest, error) {
	var m Manifest
	if err := regularFile(archive); err != nil {
		return m, err
	}
	info, err := os.Stat(archive)
	if err != nil {
		return m, err
	}
	if info.Size() > MaxDatabaseBytes+(16<<20) {
		return m, fmt.Errorf("archive exceeds size limit")
	}
	f, err := os.Open(archive)
	if err != nil {
		return m, err
	}
	defer f.Close()
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return m, err
	}
	identity.SetMaxWorkFactor(18)
	decrypted, err := age.Decrypt(f, identity)
	if err != nil {
		return m, fmt.Errorf("decrypt archive: %w", err)
	}
	// Limit all plaintext, including trailing bytes, and consume to authenticated EOF.
	limited := &io.LimitedReader{R: decrypted, N: MaxDatabaseBytes + maxManifestBytes + (1 << 20)}
	tr := tar.NewReader(limited)
	seen := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return m, err
		}
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, err
		}
		if h.Typeflag != tar.TypeReg || seen[h.Name] || (h.Name != "manifest.json" && h.Name != "identity.db") {
			return m, fmt.Errorf("invalid or duplicate archive entry")
		}
		seen[h.Name] = true
		if h.Name == "manifest.json" {
			if h.Size <= 0 || h.Size > maxManifestBytes {
				return m, fmt.Errorf("invalid manifest size")
			}
			raw, err := io.ReadAll(tr)
			if err != nil {
				return m, err
			}
			if err := json.Unmarshal(raw, &m); err != nil {
				return m, fmt.Errorf("invalid manifest: %w", err)
			}
		} else {
			if h.Size <= 0 || h.Size > MaxDatabaseBytes {
				return m, fmt.Errorf("invalid database size")
			}
			dst, err := os.OpenFile(filepath.Join(stage, "identity.db"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return m, err
			}
			_, err = io.Copy(dst, tr)
			closeErr := dst.Close()
			if err != nil {
				return m, err
			}
			if closeErr != nil {
				return m, closeErr
			}
		}
	}
	trailing, err := io.Copy(io.Discard, limited)
	if err != nil {
		return m, fmt.Errorf("archive authentication: %w", err)
	}
	if limited.N == 0 || trailing != 0 {
		return m, fmt.Errorf("unexpected trailing archive data")
	}
	if len(seen) != 2 || m.Format != Format || !reflect.DeepEqual(m.Coverage, []string{"identity"}) {
		return m, fmt.Errorf("unsupported or incomplete recovery archive")
	}
	if !reflect.DeepEqual(m.Environment, environment(cfg)) {
		return m, fmt.Errorf("issuer or WebAuthn configuration differs from backup; reuse the saved environment")
	}
	size, digest, err := fileDigest(filepath.Join(stage, "identity.db"))
	if err != nil {
		return m, err
	}
	if size != m.DatabaseBytes || digest != m.SHA256 {
		return m, fmt.Errorf("database checksum or size mismatch")
	}
	summary, err := Inspect(ctx, filepath.Join(stage, "identity.db"), cfg)
	if err != nil {
		return m, err
	}
	if !reflect.DeepEqual(summary, m.Summary) {
		return m, fmt.Errorf("manifest summary differs from database")
	}
	return m, nil
}

func regularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("expected a regular file")
	}
	return nil
}
func fileDigest(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, MaxDatabaseBytes+1))
	if err != nil {
		return 0, "", err
	}
	if n > MaxDatabaseBytes {
		return 0, "", fmt.Errorf("database exceeds size limit")
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}
