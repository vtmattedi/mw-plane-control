package httpapi

import "testing"

func TestSafeReturnTo(t *testing.T) {
	for _, value := range []string{
		"/",
		"/oauth/projects",
		"/settings?tab=security",
	} {
		if got := safeReturnTo(value); got != value {
			t.Fatalf(
				"safeReturnTo(%q) = %q",
				value,
				got,
			)
		}
	}

	for _, value := range []string{
		"https://evil.example/",
		"//evil.example/",
		`/\evil.example`,
		"relative",
	} {
		if got := safeReturnTo(value); got != "" {
			t.Fatalf(
				"unsafe return_to accepted: %q",
				value,
			)
		}
	}
}
