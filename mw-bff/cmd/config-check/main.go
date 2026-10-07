package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/vtmattedi/mw-plane-control/mw-bff/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(err.Error())
	}
	if !production(cfg.AppEnv) {
		fmt.Println("BFF configuration valid for non-production environment")
		return
	}
	var issues []string
	if !externalHTTPS(cfg.PublicBaseURL) {
		issues = append(issues, "BFF_PUBLIC_BASE_URL must be external HTTPS")
	}
	if !externalHTTPS(cfg.IdentityIssuer) {
		issues = append(issues, "BFF_IDENTITY_ISSUER must be external HTTPS")
	}
	if cfg.DatabasePath == ":memory:" {
		issues = append(issues, "DATABASE_PATH must be persistent")
	}
	if err := validateKeyring(cfg.TransactionKeys, cfg.TransactionActiveKeyVersion); err != nil {
		issues = append(issues, err.Error())
	}
	if cfg.Step09TestRSURL != "" || cfg.Step09TestAudience != "" || cfg.Step09TestScope != "" {
		issues = append(issues, "Step 09 test resource configuration must be disabled")
	}
	if cfg.OAuthBaseURL == "" || cfg.OAuthHost == "" || cfg.OAuthAudience != "mw-oauth" {
		issues = append(issues, "MW OAuth federation must be configured with audience mw-oauth")
	}
	if cfg.MailBaseURL == "" || cfg.MailHost == "" || cfg.MailAudience != "mw-mail" {
		issues = append(issues, "MW Mail federation must be configured with audience mw-mail")
	}
	// MQTT admin federation is optional; when enabled it must target audience mw-mqtt.
	if cfg.MQTTBaseURL != "" && cfg.MQTTAudience != "mw-mqtt" {
		issues = append(issues, "MW MQTT federation must use audience mw-mqtt")
	}
	if len(issues) > 0 {
		fatal(strings.Join(issues, "; "))
	}
	fmt.Println("BFF production configuration PASS")
}
func validateKeyring(raw string, active int) error {
	seen := map[int]struct{}{}
	for _, part := range strings.Split(strings.TrimSpace(raw), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, e, ok := strings.Cut(part, ":")
		if !ok {
			return fmt.Errorf("BFF_TRANSACTION_KEYS contains an invalid entry")
		}
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n <= 0 {
			return fmt.Errorf("BFF_TRANSACTION_KEYS contains an invalid version")
		}
		if _, exists := seen[n]; exists {
			return fmt.Errorf("BFF_TRANSACTION_KEYS contains duplicate version %d", n)
		}
		key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(e))
		if err != nil || len(key) != 32 {
			return fmt.Errorf("BFF_TRANSACTION_KEYS version %d must decode to 32 bytes", n)
		}
		seen[n] = struct{}{}
	}
	if len(seen) == 0 {
		return fmt.Errorf("BFF_TRANSACTION_KEYS is required")
	}
	if _, ok := seen[active]; !ok {
		return fmt.Errorf("BFF_TRANSACTION_KEYS active version %d is missing", active)
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
func fatal(m string) { fmt.Fprintln(os.Stderr, "ERROR:", m); os.Exit(1) }
