package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      uint32 = 64 * 1024
	argonIterations  uint32 = 3
	argonParallelism uint8  = 2
	argonSaltLength         = 16
	argonKeyLength   uint32 = 32
)

var (
	ErrInvalidPasswordHash = errors.New("invalid password hash")
	ErrPasswordTooShort    = errors.New("password must be at least 12 bytes")
)

func HashPassword(password []byte) (string, error) {
	if len(password) < 12 {
		return "", ErrPasswordTooShort
	}

	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := argon2.IDKey(
		password,
		salt,
		argonIterations,
		argonMemory,
		argonParallelism,
		argonKeyLength,
	)

	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(
	password []byte,
	encoded string,
) (bool, error) {
	memory, iterations, parallelism, salt, expected, err :=
		parsePasswordHash(encoded)
	if err != nil {
		return false, err
	}

	actual := argon2.IDKey(
		password,
		salt,
		iterations,
		memory,
		parallelism,
		uint32(len(expected)),
	)

	if len(actual) != len(expected) {
		return false, nil
	}

	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func parsePasswordHash(
	encoded string,
) (
	memory uint32,
	iterations uint32,
	parallelism uint8,
	salt []byte,
	key []byte,
	err error,
) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 ||
		parts[1] != "argon2id" ||
		parts[2] != "v=19" {
		err = ErrInvalidPasswordHash
		return
	}

	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		err = ErrInvalidPasswordHash
		return
	}

	if !strings.HasPrefix(params[0], "m=") ||
		!strings.HasPrefix(params[1], "t=") ||
		!strings.HasPrefix(params[2], "p=") {
		err = ErrInvalidPasswordHash
		return
	}

	memory64, parseErr := strconv.ParseUint(
		strings.TrimPrefix(params[0], "m="),
		10,
		32,
	)
	if parseErr != nil {
		err = ErrInvalidPasswordHash
		return
	}

	iterations64, parseErr := strconv.ParseUint(
		strings.TrimPrefix(params[1], "t="),
		10,
		32,
	)
	if parseErr != nil {
		err = ErrInvalidPasswordHash
		return
	}

	parallelism64, parseErr := strconv.ParseUint(
		strings.TrimPrefix(params[2], "p="),
		10,
		8,
	)
	if parseErr != nil {
		err = ErrInvalidPasswordHash
		return
	}

	salt, parseErr = base64.RawStdEncoding.DecodeString(parts[4])
	if parseErr != nil || len(salt) < 8 {
		err = ErrInvalidPasswordHash
		return
	}

	key, parseErr = base64.RawStdEncoding.DecodeString(parts[5])
	if parseErr != nil || len(key) < 16 {
		err = ErrInvalidPasswordHash
		return
	}

	memory = uint32(memory64)
	iterations = uint32(iterations64)
	parallelism = uint8(parallelism64)

	if memory == 0 || iterations == 0 || parallelism == 0 {
		err = ErrInvalidPasswordHash
	}

	return
}

func ZeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
