package auth

import "testing"

func TestNormalizeUsername(t *testing.T) {
	value, err := NormalizeUsername("  Public.User_1 ")
	if err != nil || value != "public.user_1" {
		t.Fatalf("username = %q %v", value, err)
	}
	if _, err := NormalizeUsername("no spaces allowed"); err == nil {
		t.Fatal("invalid username accepted")
	}
}
