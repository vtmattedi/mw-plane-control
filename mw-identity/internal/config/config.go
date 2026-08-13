package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                             string
	Port                               int
	DatabasePath                       string
	PublicIssuer                       string
	IdentitySessionTTL                 time.Duration
	LoginCSRFTTL                       time.Duration
	LoginRateLimitAttempts             int
	LoginRateLimitWindow               time.Duration
	SigningKeys                        string
	SigningActiveKeyVersion            int
	AuthorizationCodeTTL               time.Duration
	AccessTokenTTL                     time.Duration
	IDTokenTTL                         time.Duration
	WebAuthnRPID                       string
	WebAuthnRPName                     string
	WebAuthnOrigins                    []string
	WebAuthnCredentialKeys             string
	WebAuthnCredentialActiveKeyVersion int
	WebAuthnCeremonyTTL                time.Duration
}

func Load() (Config, error) {
	port, err := positiveInt("PORT", 7100)
	if err != nil || port > 65535 {
		if err == nil {
			err = fmt.Errorf("must be <= 65535")
		}
		return Config{}, fmt.Errorf("invalid PORT: %w", err)
	}

	sessionHours, err := positiveInt("IDENTITY_SESSION_TTL_HOURS", 168)
	if err != nil {
		return Config{}, err
	}
	loginCSRFMinutes, err := positiveInt(
		"IDENTITY_LOGIN_CSRF_TTL_MINUTES",
		10,
	)
	if err != nil {
		return Config{}, err
	}
	loginAttempts, err := positiveInt(
		"IDENTITY_LOGIN_RATE_ATTEMPTS",
		10,
	)
	if err != nil {
		return Config{}, err
	}
	loginWindowMinutes, err := positiveInt(
		"IDENTITY_LOGIN_RATE_WINDOW_MINUTES",
		5,
	)
	if err != nil {
		return Config{}, err
	}
	signingVersion, err := positiveInt(
		"IDENTITY_SIGNING_ACTIVE_KEY_VERSION",
		1,
	)
	if err != nil {
		return Config{}, err
	}
	codeMinutes, err := positiveInt(
		"IDENTITY_AUTHORIZATION_CODE_TTL_MINUTES",
		5,
	)
	if err != nil {
		return Config{}, err
	}
	accessMinutes, err := positiveInt(
		"IDENTITY_ACCESS_TOKEN_TTL_MINUTES",
		60,
	)
	if err != nil {
		return Config{}, err
	}
	idMinutes, err := positiveInt(
		"IDENTITY_ID_TOKEN_TTL_MINUTES",
		5,
	)
	if err != nil {
		return Config{}, err
	}
	webAuthnKeyVersion, err := positiveInt(
		"IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION",
		1,
	)
	if err != nil {
		return Config{}, err
	}
	webAuthnCeremonyMinutes, err := positiveInt(
		"IDENTITY_WEBAUTHN_CEREMONY_TTL_MINUTES",
		5,
	)
	if err != nil {
		return Config{}, err
	}

	databasePath := strings.TrimSpace(
		getenv("DATABASE_PATH", "./data/identity.db"),
	)
	if databasePath == "" {
		return Config{}, fmt.Errorf("DATABASE_PATH is required")
	}

	publicIssuer := strings.TrimRight(
		strings.TrimSpace(
			getenv(
				"PUBLIC_ISSUER",
				"https://identity.mattediworks.com",
			),
		),
		"/",
	)
	if publicIssuer == "" {
		return Config{}, fmt.Errorf("PUBLIC_ISSUER is required")
	}

	return Config{
		AppEnv:                  getenv("APP_ENV", "development"),
		Port:                    port,
		DatabasePath:            databasePath,
		PublicIssuer:            publicIssuer,
		IdentitySessionTTL:      time.Duration(sessionHours) * time.Hour,
		LoginCSRFTTL:            time.Duration(loginCSRFMinutes) * time.Minute,
		LoginRateLimitAttempts:  loginAttempts,
		LoginRateLimitWindow:    time.Duration(loginWindowMinutes) * time.Minute,
		SigningKeys:             strings.TrimSpace(os.Getenv("IDENTITY_SIGNING_KEYS")),
		SigningActiveKeyVersion: signingVersion,
		AuthorizationCodeTTL:    time.Duration(codeMinutes) * time.Minute,
		AccessTokenTTL:          time.Duration(accessMinutes) * time.Minute,
		IDTokenTTL:              time.Duration(idMinutes) * time.Minute,
		WebAuthnRPID: strings.TrimSpace(getenv(
			"IDENTITY_WEBAUTHN_RP_ID",
			"identity.mattediworks.com",
		)),
		WebAuthnRPName: strings.TrimSpace(getenv(
			"IDENTITY_WEBAUTHN_RP_NAME",
			"MW Identity",
		)),
		WebAuthnOrigins: []string{strings.TrimSpace(getenv(
			"IDENTITY_WEBAUTHN_ORIGINS",
			"https://identity.mattediworks.com",
		))},
		WebAuthnCredentialKeys: strings.TrimSpace(
			os.Getenv("IDENTITY_WEBAUTHN_CREDENTIAL_KEYS"),
		),
		WebAuthnCredentialActiveKeyVersion: webAuthnKeyVersion,
		WebAuthnCeremonyTTL:                time.Duration(webAuthnCeremonyMinutes) * time.Minute,
	}, nil
}

func positiveInt(key string, fallback int) (int, error) {
	raw := getenv(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func getenv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
