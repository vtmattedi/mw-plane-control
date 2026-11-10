package applications

import "testing"

func TestNavigationURIBoundaries(t *testing.T) {
	for _, value := range []string{"https://app.example/account", "https://app.example/account?intent=delete", "http://localhost:5173/account", ""} {
		if _, err := ValidateNavigationURI(value); err != nil {
			t.Fatalf("valid %q: %v", value, err)
		}
	}
	for _, value := range []string{"//evil.example", "http://app.example/account", "https://u:p@app.example", "https://app.example/#fragment", "https://app.example/?sub=123", "https://127.0.0.1/", "https://[::1]/", "javascript:alert(1)"} {
		if _, err := ValidateNavigationURI(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestProductionClosesLocalhostHTTPException(t *testing.T) {
	if _, err := ValidateNavigationURI("http://localhost:5173/account"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProductionURIs("http://localhost:5173/account"); err == nil {
		t.Fatal("development exception accepted in production")
	}
	if err := ValidateProductionURIs("https://app.example/account", ""); err != nil {
		t.Fatal(err)
	}
}

func TestProductionOIDCRedirectsAllowLocalhostHTTPOnly(t *testing.T) {
	allowed := []string{
		"http://localhost:3001/auth/callback",
		"http://localhost:5173/",
		"http://LOCALHOST:3001/auth/callback",
		"https://calc.example.com/auth/callback",
		"",
	}
	if err := ValidateProductionRedirectURIs(allowed...); err != nil {
		t.Fatalf("expected browser redirect URL list to be accepted: %v", err)
	}

	for _, rejected := range []string{
		"http://example.com/auth/callback",
		"http://localhost.evil.example/auth/callback",
		"http://localhost.localdomain/auth/callback",
		"http://127.0.0.1:3001/auth/callback",
		"http://[::1]:3001/auth/callback",
		"http://user:secret@localhost:3001/auth/callback",
		"http://localhost:3001/auth/callback#fragment",
		"javascript:alert(1)",
		"//localhost:3001/auth/callback",
	} {
		t.Run(rejected, func(t *testing.T) {
			if err := ValidateProductionRedirectURIs(rejected); err == nil {
				t.Fatalf("accepted unexpected production redirect: %q", rejected)
			}
		})
	}

	if err := ValidateProductionRedirectURIs(
		"https://calc.example.com/auth/callback",
		"http://not-localhost.example/callback",
	); err == nil {
		t.Fatal("accepted one invalid redirect among valid entries")
	}
}

func TestProductionLifecycleCallbackStillRequiresHTTPS(t *testing.T) {
	if err := ValidateProductionURIs("http://localhost:3001/auth/account-deleted"); err == nil {
		t.Fatal("production identity-deletion callback cannot target HTTP localhost")
	}
	if err := ValidateProductionURIs("https://calc.example.com/identity/account-deleted"); err != nil {
		t.Fatalf("public HTTPS lifecycle callback rejected: %v", err)
	}
}
