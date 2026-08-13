package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                      string
	Port                        int
	DatabasePath                string
	PublicBaseURL               string
	IdentityIssuer              string
	IdentityInternalBaseURL     string
	OIDCClientID                string
	OIDCClientSecret            string
	TransactionKeys             string
	TransactionActiveKeyVersion int
	OIDCTransactionTTL          time.Duration
	AdminSessionTTL             time.Duration
	OIDCClockSkew               time.Duration
	IdentityHTTPTimeout         time.Duration
}

func Load() (Config, error) {
	port, err := positiveInt("PORT", 7200)
	if err != nil || port > 65535 {
		if err == nil {
			err = fmt.Errorf("must be <= 65535")
		}
		return Config{}, fmt.Errorf("invalid PORT: %w", err)
	}

	transactionVersion, err := positiveInt(
		"BFF_TRANSACTION_ACTIVE_KEY_VERSION",
		1,
	)
	if err != nil {
		return Config{}, err
	}

	transactionMinutes, err := positiveInt(
		"BFF_OIDC_TRANSACTION_TTL_MINUTES",
		10,
	)
	if err != nil {
		return Config{}, err
	}

	sessionHours, err := positiveInt(
		"BFF_ADMIN_SESSION_TTL_HOURS",
		8,
	)
	if err != nil {
		return Config{}, err
	}

	clockSkewSeconds, err := positiveInt(
		"BFF_OIDC_CLOCK_SKEW_SECONDS",
		60,
	)
	if err != nil {
		return Config{}, err
	}

	httpTimeoutSeconds, err := positiveInt(
		"BFF_IDENTITY_HTTP_TIMEOUT_SECONDS",
		5,
	)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv: getenv(
			"APP_ENV",
			"development",
		),
		Port: port,
		DatabasePath: getenv(
			"DATABASE_PATH",
			"./data/bff.db",
		),
		PublicBaseURL: strings.TrimRight(
			strings.TrimSpace(
				os.Getenv("BFF_PUBLIC_BASE_URL"),
			),
			"/",
		),
		IdentityIssuer: strings.TrimRight(
			strings.TrimSpace(
				os.Getenv("BFF_IDENTITY_ISSUER"),
			),
			"/",
		),
		IdentityInternalBaseURL: strings.TrimRight(
			strings.TrimSpace(
				os.Getenv(
					"BFF_IDENTITY_INTERNAL_BASE_URL",
				),
			),
			"/",
		),
		OIDCClientID: strings.TrimSpace(
			os.Getenv("BFF_OIDC_CLIENT_ID"),
		),
		OIDCClientSecret: os.Getenv(
			"BFF_OIDC_CLIENT_SECRET",
		),
		TransactionKeys: strings.TrimSpace(
			os.Getenv("BFF_TRANSACTION_KEYS"),
		),
		TransactionActiveKeyVersion: transactionVersion,
		OIDCTransactionTTL:          time.Duration(transactionMinutes) * time.Minute,
		AdminSessionTTL:             time.Duration(sessionHours) * time.Hour,
		OIDCClockSkew:               time.Duration(clockSkewSeconds) * time.Second,
		IdentityHTTPTimeout:         time.Duration(httpTimeoutSeconds) * time.Second,
	}

	if strings.TrimSpace(cfg.DatabasePath) == "" {
		return Config{}, fmt.Errorf(
			"DATABASE_PATH is required",
		)
	}

	for name, value := range map[string]string{
		"BFF_PUBLIC_BASE_URL":            cfg.PublicBaseURL,
		"BFF_IDENTITY_ISSUER":            cfg.IdentityIssuer,
		"BFF_IDENTITY_INTERNAL_BASE_URL": cfg.IdentityInternalBaseURL,
		"BFF_OIDC_CLIENT_ID":             cfg.OIDCClientID,
		"BFF_OIDC_CLIENT_SECRET":         cfg.OIDCClientSecret,
		"BFF_TRANSACTION_KEYS":           cfg.TransactionKeys,
	} {
		if strings.TrimSpace(value) == "" {
			return Config{}, fmt.Errorf(
				"%s is required",
				name,
			)
		}
	}

	if err := requireAbsoluteURL(
		"BFF_PUBLIC_BASE_URL",
		cfg.PublicBaseURL,
	); err != nil {
		return Config{}, err
	}
	if err := requireAbsoluteURL(
		"BFF_IDENTITY_ISSUER",
		cfg.IdentityIssuer,
	); err != nil {
		return Config{}, err
	}
	if err := requireAbsoluteURL(
		"BFF_IDENTITY_INTERNAL_BASE_URL",
		cfg.IdentityInternalBaseURL,
	); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) CallbackURL() string {
	return c.PublicBaseURL +
		"/api/auth/callback"
}

func positiveInt(
	key string,
	fallback int,
) (int, error) {
	raw := getenv(
		key,
		strconv.Itoa(fallback),
	)
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf(
			"%s must be a positive integer",
			key,
		)
	}
	return value, nil
}

func requireAbsoluteURL(
	name string,
	value string,
) error {
	parsed, err := url.Parse(value)
	if err != nil ||
		parsed.Scheme == "" ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return fmt.Errorf(
			"%s must be an absolute base URL",
			name,
		)
	}
	return nil
}

func getenv(
	key string,
	fallback string,
) string {
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
