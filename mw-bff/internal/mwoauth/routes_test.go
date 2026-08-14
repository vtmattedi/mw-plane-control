package mwoauth

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
		{http.MethodGet, "/projects", ScopeProjectsRead, "/admin/api/projects"},
		{http.MethodPost, "/projects", ScopeProjectsWrite, "/admin/api/projects"},
		{http.MethodPatch, "/projects/42", ScopeProjectsWrite, "/admin/api/projects/42"},
		{http.MethodPost, "/projects/42/rotate-key", ScopeCredentialsRotate, "/admin/api/projects/42/rotate-key"},
		{http.MethodPost, "/projects/42/rotate-callback-secret", ScopeCredentialsRotate, "/admin/api/projects/42/rotate-callback-secret"},
		{http.MethodGet, "/projects/42/oauth/google", ScopePolicyRead, "/admin/api/projects/42/oauth/google"},
		{http.MethodPut, "/projects/42/oauth/google", ScopePolicyWrite, "/admin/api/projects/42/oauth/google"},
		{http.MethodGet, "/audit", ScopeAuditRead, "/admin/api/audit"},
		{http.MethodGet, "/audit/audit_01-abc", ScopeAuditRead, "/admin/api/audit/audit_01-abc"},
		{http.MethodGet, "/metrics", ScopeMetricsRead, "/admin/api/metrics"},
		{http.MethodGet, "/server", ScopeServerRead, "/admin/api/server"},
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
		{http.MethodDelete, "/projects/1"},
		{http.MethodGet, "/auth/me"},
		{http.MethodPost, "/auth/login"},
		{http.MethodGet, "/projects/0"},
		{http.MethodGet, "/projects/-1"},
		{http.MethodGet, "/projects/1/unknown"},
		{http.MethodGet, "/../server"},
		{http.MethodGet, "/projects//1"},
		{http.MethodGet, `/projects\\1`},
	}

	for _, tt := range tests {
		if _, err := Resolve(tt.method, tt.path); err == nil {
			t.Fatalf("Resolve(%s, %s) unexpectedly succeeded", tt.method, tt.path)
		}
	}
}
