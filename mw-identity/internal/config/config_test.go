package config

import (
	"testing"
	"time"
)

func TestProductionModeNormalization(t *testing.T) {
	t.Setenv("APP_ENV", " Production ")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppEnv != "production" {
		t.Fatal("production URI/transport policy could be skipped by environment spelling")
	}
}

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
		"IDENTITY_DELETION_DISPATCH_ENABLED",
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
	if !cfg.DeletionDispatchEnabled {
		t.Fatal("deletion dispatch must default to enabled")
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

func TestDeletionDispatchSwitch(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{"", true},
		{"true", true},
		{"false", false},
		{"0", false},
	} {
		t.Setenv("IDENTITY_DELETION_DISPATCH_ENABLED", test.value)
		cfg, err := Load()
		if err != nil || cfg.DeletionDispatchEnabled != test.want {
			t.Fatalf("%q: DeletionDispatchEnabled = %v, %v", test.value, cfg.DeletionDispatchEnabled, err)
		}
	}
	t.Setenv("IDENTITY_DELETION_DISPATCH_ENABLED", "sometimes")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid IDENTITY_DELETION_DISPATCH_ENABLED error")
	}
}

func TestPublicSignupSwitch(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{"", true},
		{"true", true},
		{"false", false},
		{"0", false},
	} {
		t.Setenv("IDENTITY_PUBLIC_SIGNUP_ENABLED", test.value)
		cfg, err := Load()
		if err != nil || cfg.PublicSignupEnabled != test.want {
			t.Fatalf("%q: PublicSignupEnabled = %v, %v", test.value, cfg.PublicSignupEnabled, err)
		}
	}

	t.Setenv("IDENTITY_PUBLIC_SIGNUP_ENABLED", "sometimes")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid IDENTITY_PUBLIC_SIGNUP_ENABLED error")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("PORT", "not-a-port")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid PORT error")
	}
}
