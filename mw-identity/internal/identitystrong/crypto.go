package identitystrong

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

type Keyring struct {
	keys          map[int][]byte
	activeVersion int
}

func NewKeyring(raw string, activeVersion int) (*Keyring, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("IDENTITY_TOTP_SECRET_KEYS is required")
	}
	if activeVersion <= 0 {
		return nil, fmt.Errorf("IDENTITY_TOTP_SECRET_ACTIVE_KEY_VERSION must be positive")
	}

	keys := make(map[int][]byte)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		versionText, encoded, ok := strings.Cut(entry, ":")
		if !ok {
			return nil, fmt.Errorf("invalid TOTP keyring entry %q", entry)
		}
		version, err := strconv.Atoi(strings.TrimSpace(versionText))
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid TOTP key version %q", versionText)
		}
		key, err := decodeKey(strings.TrimSpace(encoded))
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("TOTP key version %d must decode to 32 bytes", version)
		}
		if _, exists := keys[version]; exists {
			return nil, fmt.Errorf("duplicate TOTP key version %d", version)
		}
		keys[version] = key
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no TOTP encryption keys configured")
	}
	if _, ok := keys[activeVersion]; !ok {
		return nil, fmt.Errorf("active TOTP key version %d is not configured", activeVersion)
	}
	return &Keyring{keys: keys, activeVersion: activeVersion}, nil
}

func (k *Keyring) Encrypt(plaintext []byte, aad []byte) (string, string, int, error) {
	block, err := aes.NewCipher(k.keys[k.activeVersion])
	if err != nil {
		return "", "", 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", 0, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", 0, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, aad)
	return base64.RawURLEncoding.EncodeToString(sealed),
		base64.RawURLEncoding.EncodeToString(nonce), k.activeVersion, nil
}

func (k *Keyring) Decrypt(ciphertext, nonce string, version int, aad []byte) ([]byte, error) {
	key, ok := k.keys[version]
	if !ok {
		return nil, fmt.Errorf("TOTP key version %d is not configured", version)
	}
	sealed, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode TOTP ciphertext: %w", err)
	}
	nonceBytes, err := base64.RawURLEncoding.DecodeString(nonce)
	if err != nil {
		return nil, fmt.Errorf("decode TOTP nonce: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonceBytes) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid TOTP nonce length")
	}
	plaintext, err := gcm.Open(nil, nonceBytes, sealed, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt TOTP secret: %w", err)
	}
	return plaintext, nil
}

func decodeKey(raw string) ([]byte, error) {
	if key, err := base64.RawURLEncoding.DecodeString(raw); err == nil {
		return key, nil
	}
	return base64.StdEncoding.DecodeString(raw)
}
