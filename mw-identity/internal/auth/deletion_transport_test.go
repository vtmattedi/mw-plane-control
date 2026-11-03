package auth

import (
	"net"
	"testing"
)

func TestCallbackAddressBoundary(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.1.2.3", "169.254.169.254", "0.0.0.0", "100.64.0.1", "::1", "fc00::1", "fe80::1", "::ffff:127.0.0.1", "192.0.2.1"} {
		if publicCallbackIP(net.ParseIP(value)) {
			t.Fatalf("unsafe callback address %s", value)
		}
	}
	if !publicCallbackIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
}
