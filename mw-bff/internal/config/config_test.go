package config

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestLoadOIDCConfig(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = 7
	}

	t.Setenv(
		"BFF_PUBLIC_BASE_URL",
		"https://admin.mattediworks.com",
	)
	t.Setenv(
		"BFF_IDENTITY_ISSUER",
		"https://identity.mattediworks.com",
	)
	t.Setenv(
		"BFF_IDENTITY_INTERNAL_BASE_URL",
		"http://mw-identity:7100",
	)
	t.Setenv(
		"BFF_OIDC_CLIENT_ID",
		"mw_admin",
	)
	t.Setenv(
		"BFF_OIDC_CLIENT_SECRET",
		"secret",
	)
	t.Setenv(
		"BFF_TRANSACTION_KEYS",
		"1:"+
			base64.RawURLEncoding.EncodeToString(raw),
	)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Port != 7200 {
		t.Fatalf("Port = %d", cfg.Port)
	}
	if cfg.CallbackURL() !=
		"https://admin.mattediworks.com/api/auth/callback" {
		t.Fatalf(
			"CallbackURL = %q",
			cfg.CallbackURL(),
		)
	}
	if cfg.OIDCTransactionTTL !=
		10*time.Minute {
		t.Fatalf(
			"transaction TTL = %s",
			cfg.OIDCTransactionTTL,
		)
	}
	if cfg.AdminSessionTTL !=
		8*time.Hour {
		t.Fatalf(
			"session TTL = %s",
			cfg.AdminSessionTTL,
		)
	}
}

func TestLoadRequiresClientSecret(t *testing.T) {
	raw := make([]byte, 32)

	t.Setenv(
		"BFF_PUBLIC_BASE_URL",
		"https://admin.mattediworks.com",
	)
	t.Setenv(
		"BFF_IDENTITY_ISSUER",
		"https://identity.mattediworks.com",
	)
	t.Setenv(
		"BFF_IDENTITY_INTERNAL_BASE_URL",
		"http://mw-identity:7100",
	)
	t.Setenv(
		"BFF_OIDC_CLIENT_ID",
		"mw_admin",
	)
	t.Setenv(
		"BFF_OIDC_CLIENT_SECRET",
		"",
	)
	t.Setenv(
		"BFF_TRANSACTION_KEYS",
		"1:"+
			base64.RawURLEncoding.EncodeToString(raw),
	)

	if _, err := Load(); err == nil {
		t.Fatal("expected missing client secret error")
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	raw := make([]byte, 32)
	t.Setenv("BFF_PUBLIC_BASE_URL", "https://admin.mattediworks.com")
	t.Setenv("BFF_IDENTITY_ISSUER", "https://identity.mattediworks.com")
	t.Setenv("BFF_IDENTITY_INTERNAL_BASE_URL", "http://mw-identity:7100")
	t.Setenv("BFF_OIDC_CLIENT_ID", "mw_admin")
	t.Setenv("BFF_OIDC_CLIENT_SECRET", "secret")
	t.Setenv("BFF_TRANSACTION_KEYS", "1:"+base64.RawURLEncoding.EncodeToString(raw))
}

func TestMQTTFacadeIsOptionalAndDefaultsAreSafe(t *testing.T) {
	setRequiredEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTTBaseURL != "" {
		t.Fatalf("MQTT facade must be off without BFF_MQTT_BASE_URL, got %q", cfg.MQTTBaseURL)
	}
	if cfg.MQTTAudience != "mw-mqtt" || cfg.MQTTHTTPTimeout != 5*time.Second {
		t.Fatalf("defaults = %q / %s", cfg.MQTTAudience, cfg.MQTTHTTPTimeout)
	}
}

func TestMQTTFacadeConfigFromEnvironment(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("BFF_MQTT_BASE_URL", "https://admin-mqtt.mattediworks.com/")
	t.Setenv("BFF_MQTT_HOST", "admin-mqtt.mattediworks.com")
	t.Setenv("BFF_MQTT_HTTP_TIMEOUT_SECONDS", "9")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTTBaseURL != "https://admin-mqtt.mattediworks.com" {
		t.Fatalf("MQTTBaseURL = %q", cfg.MQTTBaseURL)
	}
	if cfg.MQTTHost != "admin-mqtt.mattediworks.com" || cfg.MQTTHTTPTimeout != 9*time.Second {
		t.Fatalf("host/timeout = %q / %s", cfg.MQTTHost, cfg.MQTTHTTPTimeout)
	}
}
