package signing

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	SigningAlgorithm = "RS256"
	rsaBits          = 2048
)

type Manager struct {
	store   Store
	keyring Keyring
	now     func() time.Time
}

type ActiveSigner struct {
	KID        string
	Algorithm  string
	PrivateKey *rsa.PrivateKey
}

type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	KID string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type JWKS struct {
	Keys []json.RawMessage `json:"keys"`
}

func NewManager(
	store Store,
	keyring Keyring,
) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("signing store is required")
	}

	return &Manager{
		store:   store,
		keyring: keyring,
		now:     time.Now,
	}, nil
}

func (m *Manager) EnsureActiveKey(
	ctx context.Context,
) (string, error) {
	key, err := m.store.ActiveSigningKey(ctx)
	if err == nil {
		return key.KID, nil
	}
	if !errors.Is(err, ErrNoActiveKey) {
		return "", err
	}

	return m.Rotate(ctx)
}

func (m *Manager) Rotate(
	ctx context.Context,
) (string, error) {
	privateKey, err := rsa.GenerateKey(
		rand.Reader,
		rsaBits,
	)
	if err != nil {
		return "", fmt.Errorf(
			"generate RSA signing key: %w",
			err,
		)
	}

	if err := privateKey.Validate(); err != nil {
		return "", fmt.Errorf(
			"validate RSA signing key: %w",
			err,
		)
	}

	kid, err := randomID("sig_")
	if err != nil {
		return "", err
	}

	publicJSON, err := marshalPublicJWK(
		kid,
		&privateKey.PublicKey,
	)
	if err != nil {
		return "", err
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(
		privateKey,
	)
	if err != nil {
		return "", fmt.Errorf(
			"marshal private signing key: %w",
			err,
		)
	}
	defer zero(privateDER)

	version, encryptionKey := m.keyring.Active()
	defer zero(encryptionKey)

	ciphertext, nonce, err := encryptPrivateKey(
		encryptionKey,
		kid,
		privateDER,
	)
	if err != nil {
		return "", err
	}

	now := m.now().UTC()
	record := StoredKey{
		KID:                  kid,
		Alg:                  SigningAlgorithm,
		Status:               "active",
		PublicJWKJSON:        publicJSON,
		PrivateCiphertext:    ciphertext,
		PrivateNonce:         nonce,
		EncryptionKeyVersion: version,
		CreatedAt:            now,
	}

	if err := m.store.RotateSigningKey(
		ctx,
		record,
		now,
	); err != nil {
		return "", err
	}

	return kid, nil
}

func (m *Manager) ActiveSigner(
	ctx context.Context,
) (ActiveSigner, error) {
	record, err := m.store.ActiveSigningKey(ctx)
	if err != nil {
		return ActiveSigner{}, err
	}

	key, ok := m.keyring.Key(
		record.EncryptionKeyVersion,
	)
	if !ok {
		return ActiveSigner{}, fmt.Errorf(
			"signing encryption key version %d is unavailable",
			record.EncryptionKeyVersion,
		)
	}
	defer zero(key)

	privateDER, err := decryptPrivateKey(
		key,
		record.KID,
		record.PrivateNonce,
		record.PrivateCiphertext,
	)
	if err != nil {
		return ActiveSigner{}, err
	}
	defer zero(privateDER)

	parsed, err := x509.ParsePKCS8PrivateKey(privateDER)
	if err != nil {
		return ActiveSigner{}, fmt.Errorf(
			"parse private signing key: %w",
			err,
		)
	}

	privateKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return ActiveSigner{}, fmt.Errorf(
			"stored signing key is not RSA",
		)
	}

	return ActiveSigner{
		KID:        record.KID,
		Algorithm:  record.Alg,
		PrivateKey: privateKey,
	}, nil
}

func (m *Manager) PublicJWKS(
	ctx context.Context,
) (JWKS, error) {
	records, err := m.store.PublicSigningKeys(ctx)
	if err != nil {
		return JWKS{}, err
	}

	result := JWKS{
		Keys: make(
			[]json.RawMessage,
			0,
			len(records),
		),
	}

	for _, record := range records {
		var value json.RawMessage
		if err := json.Unmarshal(
			[]byte(record.PublicJWKJSON),
			&value,
		); err != nil {
			return JWKS{}, fmt.Errorf(
				"decode public JWK %s: %w",
				record.KID,
				err,
			)
		}

		result.Keys = append(result.Keys, value)
	}

	return result, nil
}

func marshalPublicJWK(
	kid string,
	key *rsa.PublicKey,
) (string, error) {
	exponent := make([]byte, 4)
	binary.BigEndian.PutUint32(
		exponent,
		uint32(key.E),
	)
	exponent = trimLeadingZeroes(exponent)

	value := jwk{
		Kty: "RSA",
		Use: "sig",
		Alg: SigningAlgorithm,
		KID: kid,
		N: base64.RawURLEncoding.EncodeToString(
			key.N.Bytes(),
		),
		E: base64.RawURLEncoding.EncodeToString(
			exponent,
		),
	}

	body, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf(
			"marshal public JWK: %w",
			err,
		)
	}

	return string(body), nil
}

func encryptPrivateKey(
	key []byte,
	kid string,
	plaintext []byte,
) ([]byte, []byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf(
			"generate signing-key nonce: %w",
			err,
		)
	}

	ciphertext := gcm.Seal(
		nil,
		nonce,
		plaintext,
		[]byte(signingAAD(kid)),
	)

	return ciphertext, nonce, nil
}

func decryptPrivateKey(
	key []byte,
	kid string,
	nonce []byte,
	ciphertext []byte,
) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(
		nil,
		nonce,
		ciphertext,
		[]byte(signingAAD(kid)),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"decrypt private signing key: %w",
			err,
		)
	}

	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf(
			"create signing-key AES cipher: %w",
			err,
		)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf(
			"create signing-key GCM: %w",
			err,
		)
	}

	return gcm, nil
}

func signingAAD(kid string) string {
	return "mw-identity-signing-key:" + kid
}

func randomID(prefix string) (string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf(
			"generate signing identifier: %w",
			err,
		)
	}

	return prefix +
		base64.RawURLEncoding.EncodeToString(raw), nil
}

func trimLeadingZeroes(value []byte) []byte {
	index := 0
	for index < len(value)-1 &&
		value[index] == 0 {
		index++
	}
	return value[index:]
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
