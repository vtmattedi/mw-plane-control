package signing

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testEncodedKey(fill byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = fill
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func TestParseKeyring(t *testing.T) {
	value := "1:" + testEncodedKey(1) +
		",2:" + testEncodedKey(2)

	keyring, err := ParseKeyring(value, 2)
	if err != nil {
		t.Fatal(err)
	}

	version, key := keyring.Active()
	if version != 2 {
		t.Fatalf("active version = %d", version)
	}
	if len(key) != 32 {
		t.Fatalf("active key length = %d", len(key))
	}
}

func TestParseKeyringRejectsWrongLength(t *testing.T) {
	value := "1:" + base64.RawURLEncoding.EncodeToString(
		[]byte("short"),
	)

	_, err := ParseKeyring(value, 1)
	if err == nil ||
		!strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("error = %v", err)
	}
}
