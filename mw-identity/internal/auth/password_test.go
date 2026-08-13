package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	password := []byte("correct horse battery staple")

	encoded, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("unexpected hash prefix: %q", encoded)
	}

	ok, err := VerifyPassword(password, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("correct password did not verify")
	}

	ok, err = VerifyPassword([]byte("wrong-password-value"), encoded)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("wrong password verified")
	}
}

func TestPasswordMinimumLength(t *testing.T) {
	_, err := HashPassword([]byte("too-short"))
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("error = %v", err)
	}
}
