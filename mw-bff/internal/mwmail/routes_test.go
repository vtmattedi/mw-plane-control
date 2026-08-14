package mwmail

import (
	"net/http"
	"testing"
)

func TestResolveKnownRoutes(t *testing.T) {
	tests := []struct {
		method string
		path   string
		scope  string
		target string
	}{
		{http.MethodGet, "/health", ScopeProjectsRead, "/admin/api/health"},
		{http.MethodGet, "/projects", ScopeProjectsRead, "/admin/api/projects"},
		{http.MethodPost, "/projects", ScopeProjectsWrite, "/admin/api/projects"},
		{http.MethodPatch, "/projects/42", ScopeProjectsWrite, "/admin/api/projects/42"},
		{http.MethodDelete, "/projects/42", ScopeProjectsDelete, "/admin/api/projects/42"},
		{http.MethodGet, "/projects/42/request-domains", ScopeProjectsRead, "/admin/api/projects/42/request-domains"},
		{http.MethodPut, "/projects/42/request-domains", ScopeProjectsWrite, "/admin/api/projects/42/request-domains"},
		{http.MethodPost, "/projects/42/rotate-key", ScopeCredentialsRotate, "/admin/api/projects/42/rotate-key"},
		{http.MethodGet, "/emails", ScopeEmailsRead, "/admin/api/emails"},
		{http.MethodGet, "/emails/71", ScopeEmailsRead, "/admin/api/emails/71"},
		{http.MethodGet, "/emails/by-project", ScopeUsageRead, "/admin/api/emails/by-project"},
		{http.MethodGet, "/docs", ScopeServerRead, "/admin/api/docs"},
		{http.MethodGet, "/docs/v1", ScopeServerRead, "/admin/api/docs/v1"},
	}

	for _, tt := range tests {
		route, err := Resolve(tt.method, tt.path)
		if err != nil {
			t.Fatalf("Resolve(%s, %s): %v", tt.method, tt.path, err)
		}
		if route.Scope != tt.scope || route.DownstreamPath != tt.target {
			t.Fatalf("Resolve(%s, %s) = %#v", tt.method, tt.path, route)
		}
	}
}

func TestResolveRejectsUnmappedAndTraversalRoutes(t *testing.T) {
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/auth/me"},
		{http.MethodPost, "/auth/login"},
		{http.MethodDelete, "/projects/0"},
		{http.MethodGet, "/projects/0"},
		{http.MethodGet, "/projects/-1"},
		{http.MethodGet, "/projects/1/unknown"},
		{http.MethodPost, "/emails"},
		{http.MethodGet, "/emails/0"},
		{http.MethodGet, "/../projects"},
		{http.MethodGet, "/projects//1"},
		{http.MethodGet, `/projects\\1`},
	}

	for _, tt := range tests {
		if _, err := Resolve(tt.method, tt.path); err == nil {
			t.Fatalf("Resolve(%s, %s) unexpectedly succeeded", tt.method, tt.path)
		}
	}
}
