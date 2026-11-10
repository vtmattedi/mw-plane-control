package applications

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateNavigationURI allows static HTTPS navigation and development localhost.
// It grants no action authority and never appends identity or credential values.
func ValidateNavigationURI(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	u, err := url.Parse(value)
	if err != nil || len(value) > 2048 || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && u.Hostname() == "localhost")) {
		return "", fmt.Errorf("invalid HTTPS navigation URI")
	}
	for k := range u.Query() {
		switch strings.ToLower(k) {
		case "sub", "user_id", "token", "access_token", "id_token", "code", "state", "secret":
			return "", fmt.Errorf("identity/credential parameters forbidden")
		}
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast()) {
		return "", fmt.Errorf("private address forbidden")
	}
	return value, nil
}

// ValidateProductionURIs requires HTTPS for production server-side callbacks
// and application navigation links. Loopback redirects are validated separately.
func ValidateProductionURIs(values ...string) error {
	for _, value := range values {
		if value == "" {
			continue
		}
		u, err := url.Parse(value)
		if err != nil || len(value) > 2048 || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("production URI requires HTTPS")
		}
	}
	return nil
}


// ValidateProductionRedirectURIs allows OAuth's exact-match loopback development
// exception for browser redirects, even when the Identity provider runs in
// production. These URLs are sent to a user's browser, not fetched by Identity.
// Do not use this function for server-to-server lifecycle callbacks.
func ValidateProductionRedirectURIs(values ...string) error {
	for _, value := range values {
		if value == "" {
			continue
		}
		u, err := url.Parse(value)
		if err != nil || len(value) > 2048 || u.Host == "" || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("invalid production OIDC redirect URI")
		}
		if u.Scheme == "https" ||
			(u.Scheme == "http" && strings.EqualFold(u.Hostname(), "localhost")) {
			continue
		}
		return fmt.Errorf("production OIDC redirect URI requires HTTPS outside localhost")
	}
	return nil
}
