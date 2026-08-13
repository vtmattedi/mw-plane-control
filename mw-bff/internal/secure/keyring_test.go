package secure

import (
	"encoding/base64"
	"testing"
)

func TestKeyringEncryptRoundTrip(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = 42
	}

	keyring, err := ParseKeyring(
		"1:"+
			base64.RawURLEncoding.EncodeToString(raw),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("pkce-secret")
	ciphertext, nonce, version, err := keyring.Encrypt(
		"transaction:test",
		plaintext,
	)
	if err != nil {
		t.Fatal(err)
	}

	recovered, err := keyring.Decrypt(
		version,
		"transaction:test",
		nonce,
		ciphertext,
	)
	if err != nil {
		t.Fatal(err)
	}

	if string(recovered) != string(plaintext) {
		t.Fatalf(
			"recovered = %q",
			string(recovered),
		)
	}
}

func TestKeyringRejectsWrongAAD(t *testing.T) {
	raw := make([]byte, 32)
	keyring, err := ParseKeyring(
		"1:"+
			base64.RawURLEncoding.EncodeToString(raw),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, nonce, version, err := keyring.Encrypt(
		"expected",
		[]byte("secret"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := keyring.Decrypt(
		version,
		"wrong",
		nonce,
		ciphertext,
	); err == nil {
		t.Fatal("wrong AAD decrypted successfully")
	}
}
