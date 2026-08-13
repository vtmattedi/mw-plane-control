package httpapi

import "testing"

func TestSafeReturnTo(t *testing.T) {
	for _, value := range []string{
		"/account",
		"/oauth/authorize?client_id=test",
	} {
		if got := safeReturnTo(value); got != value {
			t.Fatalf("safeReturnTo(%q) = %q", value, got)
		}
	}

	for _, value := range []string{
		"https://evil.example/",
		"//evil.example/path",
		`/\evil.example/path`,
		"relative/path",
	} {
		if got := safeReturnTo(value); got != "" {
			t.Fatalf("unsafe return_to accepted: %q -> %q", value, got)
		}
	}
}
