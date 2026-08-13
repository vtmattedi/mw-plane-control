package httpapi

import (
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	limiter := newLoginLimiter(2, time.Minute)
	now := time.Unix(1000, 0)
	limiter.now = func() time.Time {
		return now
	}

	if !limiter.Allow("ip") {
		t.Fatal("first attempt was rejected")
	}
	if !limiter.Allow("ip") {
		t.Fatal("second attempt was rejected")
	}
	if limiter.Allow("ip") {
		t.Fatal("third attempt should be rate limited")
	}

	now = now.Add(2 * time.Minute)
	if !limiter.Allow("ip") {
		t.Fatal("attempt after window should be allowed")
	}
}
