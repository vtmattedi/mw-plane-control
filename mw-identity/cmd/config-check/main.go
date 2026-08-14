package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(err.Error())
	}
	if !production(cfg.AppEnv) {
		fmt.Println("Identity configuration valid for non-production environment")
		return
	}
	var issues []string
	if !externalHTTPS(cfg.PublicIssuer) {
		issues = append(issues, "PUBLIC_ISSUER must be external HTTPS")
	}
	if cfg.DatabasePath == ":memory:" {
		issues = append(issues, "DATABASE_PATH must be persistent")
	}
	if err := validateKeyring("IDENTITY_SIGNING_KEYS", cfg.SigningKeys, cfg.SigningActiveKeyVersion); err != nil {
		issues = append(issues, err.Error())
	}
	if err := validateKeyring("IDENTITY_WEBAUTHN_CREDENTIAL_KEYS", cfg.WebAuthnCredentialKeys, cfg.WebAuthnCredentialActiveKeyVersion); err != nil {
		issues = append(issues, err.Error())
	}
	if err := validateKeyring("IDENTITY_TOTP_SECRET_KEYS", cfg.TOTPSecretKeys, cfg.TOTPSecretActiveKeyVersion); err != nil {
		issues = append(issues, err.Error())
	}
	if localHost(cfg.WebAuthnRPID) {
		issues = append(issues, "IDENTITY_WEBAUTHN_RP_ID must not be localhost")
	}
	for _, origin := range cfg.WebAuthnOrigins {
		if !externalHTTPS(origin) {
			issues = append(issues, "all IDENTITY_WEBAUTHN_ORIGINS must be external HTTPS")
			break
		}
	}
	retired := envInt("IDENTITY_SIGNING_RETIRED_RETENTION_HOURS", 72)
	minimum := int(cfg.AccessTokenTTL.Hours()) + 1
	if retired < minimum {
		issues = append(issues, fmt.Sprintf("IDENTITY_SIGNING_RETIRED_RETENTION_HOURS must be >= %d", minimum))
	}
	if len(issues) > 0 {
		fatal(strings.Join(issues, "; "))
	}
	fmt.Println("Identity production configuration PASS")
}

func validateKeyring(name, raw string, active int) error {
	seen := map[int]struct{}{}
	for _, part := range strings.Split(strings.TrimSpace(raw), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		versionText, encoded, ok := strings.Cut(part, ":")
		if !ok {
			return fmt.Errorf("%s contains an invalid entry", name)
		}
		version, err := strconv.Atoi(strings.TrimSpace(versionText))
		if err != nil || version <= 0 {
			return fmt.Errorf("%s contains an invalid version", name)
		}
		if _, exists := seen[version]; exists {
			return fmt.Errorf("%s contains duplicate version %d", name, version)
		}
		key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
		if err != nil {
			key, err = base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
		}
		if err != nil || len(key) != 32 {
			return fmt.Errorf("%s version %d must decode to 32 bytes", name, version)
		}
		seen[version] = struct{}{}
	}
	if len(seen) == 0 {
		return fmt.Errorf("%s is required", name)
	}
	if _, ok := seen[active]; !ok {
		return fmt.Errorf("%s active version %d is missing", name, active)
	}
	return nil
}
func production(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "production" || v == "prod"
}
func externalHTTPS(raw string) bool {
	u, e := url.Parse(strings.TrimSpace(raw))
	return e == nil && u.Scheme == "https" && u.Host != "" && !localHost(u.Hostname()) && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}
func localHost(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "localhost" || v == "127.0.0.1" || v == "::1"
}
func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	n, e := strconv.Atoi(raw)
	if e != nil || n <= 0 {
		fatal(name + " must be positive")
	}
	return n
}
func fatal(m string) { fmt.Fprintln(os.Stderr, "ERROR:", m); os.Exit(1) }
