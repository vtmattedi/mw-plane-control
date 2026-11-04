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
