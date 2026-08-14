package mwmail

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientSendsOnlyFederatedServiceHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "mailgo.mattediworks.com" {
			t.Fatalf("Host = %q", r.Host)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer mwdt_example" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("Origin"); got != "" {
			t.Fatalf("Origin was forwarded: %q", got)
		}
		if got := r.Header.Get("Cookie"); got != "" {
			t.Fatalf("Cookie was forwarded: %q", got)
		}
		if got := r.Header.Get("X-MW-CSRF"); got != "" {
			t.Fatalf("X-MW-CSRF was forwarded: %q", got)
		}
		if r.URL.Path != "/admin/api/projects" || r.URL.RawQuery != "limit=10" {
			t.Fatalf("request target = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		if got := r.Header.Get("X-Request-ID"); got != "req_123" {
			t.Fatalf("X-Request-ID = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"projects":[]}`)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:     server.URL,
		Host:        "mailgo.mattediworks.com",
		HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(
		context.Background(),
		http.MethodGet,
		"/admin/api/projects",
		"limit=10",
		"mwdt_example",
		"",
		"application/json",
		"req_123",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK ||
		!strings.Contains(string(response.Body), "projects") {
		t.Fatalf("response = %#v", response)
	}
}

func TestClientRejectsRedirectFollowingAndOversize(t *testing.T) {
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.invalid/", http.StatusFound)
	}))
	defer redirect.Close()

	client, err := New(Config{
		BaseURL:          redirect.URL,
		HTTPTimeout:      time.Second,
		MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(
		context.Background(),
		http.MethodGet,
		"/admin/api/health",
		"",
		"token",
		"",
		"",
		"",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusFound {
		t.Fatalf("redirect status = %d", response.StatusCode)
	}

	large := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "0123456789")
	}))
	defer large.Close()
	client, err = New(Config{
		BaseURL:          large.URL,
		HTTPTimeout:      time.Second,
		MaxResponseBytes: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(
		context.Background(),
		http.MethodGet,
		"/admin/api/health",
		"",
		"token",
		"",
		"",
		"",
		nil,
	); err != ErrResponseTooLarge {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestNewRejectsUnsafeLogicalHost(t *testing.T) {
	if _, err := New(Config{
		BaseURL:     "https://example.com",
		Host:        "bad/host",
		HTTPTimeout: time.Second,
	}); err == nil {
		t.Fatal("unsafe logical Host unexpectedly accepted")
	}
}

func TestNewRejectsUnsafeBaseURL(t *testing.T) {
	for _, value := range []string{
		"",
		"ftp://example.com",
		"https://user:pass@example.com",
		"https://example.com/base",
		"https://example.com/?x=1",
	} {
		if _, err := New(Config{
			BaseURL:     value,
			HTTPTimeout: time.Second,
		}); err == nil {
			t.Fatalf("New(%q) unexpectedly succeeded", value)
		}
	}
}
