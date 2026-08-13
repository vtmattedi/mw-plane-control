package httpapi

import (
	"sync"
	"time"
)

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
	now      func() time.Time
}

func newLoginLimiter(
	max int,
	window time.Duration,
) *loginLimiter {
	return &loginLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
		now:      time.Now,
	}
}

func (l *loginLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	existing := l.attempts[key]
	filtered := existing[:0]
	for _, attempt := range existing {
		if attempt.After(cutoff) {
			filtered = append(filtered, attempt)
		}
	}

	if len(filtered) >= l.max {
		l.attempts[key] = filtered
		return false
	}

	filtered = append(filtered, now)
	l.attempts[key] = filtered
	return true
}

func (l *loginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, key)
}
