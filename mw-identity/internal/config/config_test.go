package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{
		"PORT",
		"DATABASE_PATH",
		"APP_ENV",
		"PUBLIC_ISSUER",
		"IDENTITY_SESSION_TTL_HOURS",
		"IDENTITY_LOGIN_CSRF_TTL_MINUTES",
		"IDENTITY_LOGIN_RATE_ATTEMPTS",
		"IDENTITY_LOGIN_RATE_WINDOW_MINUTES",
		"IDENTITY_SIGNING_KEYS",
		"IDENTITY_SIGNING_ACTIVE_KEY_VERSION",
	} {
		t.Setenv(key, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Port != 7100 {
		t.Fatalf("Port = %d", cfg.Port)
	}
	if cfg.DatabasePath != "./data/identity.db" {
		t.Fatalf(
			"DatabasePath = %q",
			cfg.DatabasePath,
		)
	}
	if cfg.PublicIssuer !=
		"https://identity.mattediworks.com" {
		t.Fatalf(
			"PublicIssuer = %q",
			cfg.PublicIssuer,
		)
	}
	if cfg.IdentitySessionTTL !=
		168*time.Hour {
		t.Fatalf(
			"IdentitySessionTTL = %s",
			cfg.IdentitySessionTTL,
		)
	}
	if cfg.SigningActiveKeyVersion != 1 {
		t.Fatalf(
			"SigningActiveKeyVersion = %d",
			cfg.SigningActiveKeyVersion,
		)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("PORT", "not-a-port")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid PORT error")
	}
}
