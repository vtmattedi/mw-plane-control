package secure

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

func ParseKeyring(
	value string,
	activeVersion int,
) (Keyring, error) {
	if activeVersion <= 0 {
		return Keyring{}, fmt.Errorf(
			"active key version must be positive",
		)
	}

	keys := make(map[int][]byte)

	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		versionRaw, encoded, ok := strings.Cut(part, ":")
		if !ok {
			return Keyring{}, fmt.Errorf(
				"invalid keyring entry %q",
				part,
			)
		}

		version, err := strconv.Atoi(
			strings.TrimSpace(versionRaw),
		)
		if err != nil || version <= 0 {
			return Keyring{}, fmt.Errorf(
				"invalid keyring version %q",
				versionRaw,
			)
		}

		key, err := base64.RawURLEncoding.DecodeString(
			strings.TrimSpace(encoded),
		)
		if err != nil {
			return Keyring{}, fmt.Errorf(
				"decode keyring version %d: %w",
				version,
				err,
			)
		}
		if len(key) != 32 {
			return Keyring{}, fmt.Errorf(
				"keyring version %d must decode to 32 bytes",
				version,
			)
		}
		if _, exists := keys[version]; exists {
			return Keyring{}, fmt.Errorf(
				"duplicate keyring version %d",
				version,
			)
		}

		keys[version] = key
	}

	if len(keys) == 0 {
		return Keyring{}, fmt.Errorf(
			"BFF_TRANSACTION_KEYS is required",
		)
	}
	if _, ok := keys[activeVersion]; !ok {
		return Keyring{}, fmt.Errorf(
			"active keyring version %d is missing",
			activeVersion,
		)
	}

	return Keyring{
		keys:          keys,
		activeVersion: activeVersion,
	}, nil
}

func (k Keyring) Encrypt(
	aad string,
	plaintext []byte,
) (
	ciphertext []byte,
	nonce []byte,
	version int,
	err error,
) {
	key := append(
		[]byte(nil),
		k.keys[k.activeVersion]...,
	)
	defer zero(key)

	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, 0, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, 0, fmt.Errorf(
			"generate encryption nonce: %w",
			err,
		)
	}

	ciphertext = gcm.Seal(
		nil,
		nonce,
		plaintext,
		[]byte(aad),
	)

	return ciphertext, nonce, k.activeVersion, nil
}

func (k Keyring) Decrypt(
	version int,
	aad string,
	nonce []byte,
	ciphertext []byte,
) ([]byte, error) {
	raw, ok := k.keys[version]
	if !ok {
		return nil, fmt.Errorf(
			"keyring version %d is unavailable",
			version,
		)
	}

	key := append([]byte(nil), raw...)
	defer zero(key)

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(
		nil,
		nonce,
		ciphertext,
		[]byte(aad),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"decrypt protected value: %w",
			err,
		)
	}

	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf(
			"create AES cipher: %w",
			err,
		)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf(
			"create AES-GCM: %w",
			err,
		)
	}

	return gcm, nil
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
