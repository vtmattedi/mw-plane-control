package signing

import (
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
			"signing encryption active key version must be positive",
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
				"invalid signing keyring entry %q",
				part,
			)
		}

		version, err := strconv.Atoi(
			strings.TrimSpace(versionRaw),
		)
		if err != nil || version <= 0 {
			return Keyring{}, fmt.Errorf(
				"invalid signing keyring version %q",
				versionRaw,
			)
		}

		key, err := base64.RawURLEncoding.DecodeString(
			strings.TrimSpace(encoded),
		)
		if err != nil {
			return Keyring{}, fmt.Errorf(
				"decode signing keyring version %d: %w",
				version,
				err,
			)
		}
		if len(key) != 32 {
			return Keyring{}, fmt.Errorf(
				"signing keyring version %d must decode to 32 bytes",
				version,
			)
		}
		if _, exists := keys[version]; exists {
			return Keyring{}, fmt.Errorf(
				"duplicate signing keyring version %d",
				version,
			)
		}

		keys[version] = key
	}

	if len(keys) == 0 {
		return Keyring{}, fmt.Errorf(
			"IDENTITY_SIGNING_KEYS is required",
		)
	}

	if _, ok := keys[activeVersion]; !ok {
		return Keyring{}, fmt.Errorf(
			"active signing encryption key version %d is missing",
			activeVersion,
		)
	}

	return Keyring{
		keys:          keys,
		activeVersion: activeVersion,
	}, nil
}

func (k Keyring) Active() (int, []byte) {
	key := k.keys[k.activeVersion]
	copyKey := append([]byte(nil), key...)
	return k.activeVersion, copyKey
}

func (k Keyring) Key(version int) ([]byte, bool) {
	key, ok := k.keys[version]
	if !ok {
		return nil, false
	}

	return append([]byte(nil), key...), true
}
