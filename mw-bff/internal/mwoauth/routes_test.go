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
		{http.MethodDelete, "/projects/42", ScopeProjectsDelete, "/admin/api/projects/42"},
		{http.MethodGet, "/projects/summary", ScopeProjectsRead, "/admin/api/projects/summary"},
		{http.MethodGet, "/projects/42/client", ScopeCredentialsRead, "/admin/api/projects/42/client"},
		{http.MethodPost, "/projects/42/client/credentials", ScopeCredentialsRotate, "/admin/api/projects/42/client/credentials"},
		{http.MethodDelete, "/projects/42/client/credentials/7", ScopeCredentialsRotate, "/admin/api/projects/42/client/credentials/7"},
		{http.MethodPost, "/projects/42/client/tokens/revoke", ScopeCredentialsRotate, "/admin/api/projects/42/client/tokens/revoke"},
		{http.MethodGet, "/projects/42/oauth/google/client", ScopeProviderCredentialsRead, "/admin/api/projects/42/oauth/google/client"},
		{http.MethodPut, "/projects/42/oauth/google/client", ScopeProviderCredentialsManage, "/admin/api/projects/42/oauth/google/client"},
		{http.MethodGet, "/projects/42/oauth/google/client/gcred_abc-123/impact", ScopeProviderCredentialsRead, "/admin/api/projects/42/oauth/google/client/gcred_abc-123/impact"},
		{http.MethodPost, "/projects/42/oauth/google/client/gcred_abc-123/retire", ScopeProviderCredentialsManage, "/admin/api/projects/42/oauth/google/client/gcred_abc-123/retire"},
		{http.MethodPost, "/projects/42/oauth/google/client/gcred_abc-123/disable", ScopeProviderCredentialsManage, "/admin/api/projects/42/oauth/google/client/gcred_abc-123/disable"},
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
		{http.MethodDelete, "/projects/0"},
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

// Google client credentials are never reachable with a policy scope, and the
// MW client lifecycle is never reachable with a read scope.
func TestCredentialRoutesNeverUsePolicyScopes(t *testing.T) {
	for _, path := range []string{
		"/projects/42/oauth/google/client",
		"/projects/42/oauth/google/client/gcred_x/retire",
		"/projects/42/oauth/google/client/gcred_x/disable",
	} {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPost} {
			route, err := Resolve(method, path)
			if err != nil {
				continue
			}
			if route.Scope == ScopePolicyRead || route.Scope == ScopePolicyWrite {
				t.Fatalf("%s %s maps to policy scope %s", method, path, route.Scope)
			}
		}
	}

	for _, path := range []string{
		"/projects/42/client/credentials",
		"/projects/42/client/tokens/revoke",
	} {
		route, err := Resolve(http.MethodPost, path)
		if err != nil || route.Scope != ScopeCredentialsRotate {
			t.Fatalf("POST %s = %#v, %v", path, route, err)
		}
	}
}
