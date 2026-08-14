package delegation

import "testing"

func TestCanonicalScope(t *testing.T) {
	got := canonicalScope([]string{"z.write", "a.read", "z.write", ""})
	if got != "a.read z.write" {
		t.Fatalf("canonical scope = %q", got)
	}
}
