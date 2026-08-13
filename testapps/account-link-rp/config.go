package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type LocalCredential struct {
	ID       string
	Email    string
	Password string
}

type Config struct {
	Addr                 string
	PublicBaseURL        string
	IdentityIssuer       string
	IdentityBrowserBase  string
	IdentityInternalBase string
	IdentityHost         string
	ClientID             string
	ClientSecret         string
	CookieSecure         bool
	TestMode             bool
	SessionTTL           time.Duration
	TransactionTTL       time.Duration
	PendingTTL           time.Duration
	Users                []LocalCredential
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Addr:                 getenv("LINK_RP_ADDR", "127.0.0.1:7350"),
		PublicBaseURL:        strings.TrimRight(getenv("LINK_RP_PUBLIC_BASE_URL", "http://127.0.0.1:7350"), "/"),
		IdentityIssuer:       strings.TrimRight(getenv("LINK_RP_IDENTITY_ISSUER", ""), "/"),
		IdentityBrowserBase:  strings.TrimRight(getenv("LINK_RP_IDENTITY_BROWSER_BASE", ""), "/"),
		IdentityInternalBase: strings.TrimRight(getenv("LINK_RP_IDENTITY_INTERNAL_BASE", ""), "/"),
		IdentityHost:         strings.TrimSpace(os.Getenv("LINK_RP_IDENTITY_HOST")),
		ClientID:             strings.TrimSpace(os.Getenv("LINK_RP_CLIENT_ID")),
		ClientSecret:         os.Getenv("LINK_RP_CLIENT_SECRET"),
		CookieSecure:         envBool("LINK_RP_COOKIE_SECURE", false),
		TestMode:             envBool("LINK_RP_TEST_MODE", false),
		SessionTTL:           time.Duration(envPositiveInt("LINK_RP_SESSION_TTL_MINUTES", 60)) * time.Minute,
		TransactionTTL:       time.Duration(envPositiveInt("LINK_RP_TRANSACTION_TTL_MINUTES", 5)) * time.Minute,
		PendingTTL:           time.Duration(envPositiveInt("LINK_RP_PENDING_TTL_MINUTES", 5)) * time.Minute,
		Users: []LocalCredential{
			{
				ID:       strings.TrimSpace(os.Getenv("LINK_RP_USER1_ID")),
				Email:    strings.TrimSpace(os.Getenv("LINK_RP_USER1_EMAIL")),
				Password: os.Getenv("LINK_RP_USER1_PASSWORD"),
			},
			{
				ID:       strings.TrimSpace(os.Getenv("LINK_RP_USER2_ID")),
				Email:    strings.TrimSpace(os.Getenv("LINK_RP_USER2_EMAIL")),
				Password: os.Getenv("LINK_RP_USER2_PASSWORD"),
			},
		},
	}

	for name, value := range map[string]string{
		"LINK_RP_IDENTITY_ISSUER":        cfg.IdentityIssuer,
		"LINK_RP_IDENTITY_BROWSER_BASE":  cfg.IdentityBrowserBase,
		"LINK_RP_IDENTITY_INTERNAL_BASE": cfg.IdentityInternalBase,
		"LINK_RP_CLIENT_ID":              cfg.ClientID,
		"LINK_RP_CLIENT_SECRET":          cfg.ClientSecret,
	} {
		if strings.TrimSpace(value) == "" {
			return Config{}, fmt.Errorf("%s is required", name)
		}
	}

	for name, raw := range map[string]string{
		"LINK_RP_PUBLIC_BASE_URL":        cfg.PublicBaseURL,
		"LINK_RP_IDENTITY_ISSUER":        cfg.IdentityIssuer,
		"LINK_RP_IDENTITY_BROWSER_BASE":  cfg.IdentityBrowserBase,
		"LINK_RP_IDENTITY_INTERNAL_BASE": cfg.IdentityInternalBase,
	} {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return Config{}, fmt.Errorf("%s must be an absolute URL", name)
		}
	}

	seenIDs := map[string]struct{}{}
	seenEmails := map[string]struct{}{}
	for index, user := range cfg.Users {
		prefix := fmt.Sprintf("LINK_RP_USER%d", index+1)
		if user.ID == "" || user.Email == "" || user.Password == "" {
			return Config{}, fmt.Errorf("%s_ID, %s_EMAIL and %s_PASSWORD are required", prefix, prefix, prefix)
		}
		normalized := normalizeEmail(user.Email)
		if _, ok := seenIDs[user.ID]; ok {
			return Config{}, fmt.Errorf("duplicate local user id %q", user.ID)
		}
		if _, ok := seenEmails[normalized]; ok {
			return Config{}, fmt.Errorf("duplicate local user email %q", normalized)
		}
		seenIDs[user.ID] = struct{}{}
		seenEmails[normalized] = struct{}{}
	}

	return cfg, nil
}

func getenv(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envPositiveInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
