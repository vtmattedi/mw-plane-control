package mwmqtt

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
		{http.MethodGet, "/server", ScopeServerRead, "/admin/api/server"},
		{http.MethodGet, "/stats", ScopeServerRead, "/admin/api/stats"},
		{http.MethodGet, "/roles", ScopeRolesRead, "/admin/api/roles"},
		{http.MethodGet, "/docs", ScopeServerRead, "/admin/api/docs"},
		{http.MethodPost, "/roles", ScopeRolesWrite, "/admin/api/roles"},
		{http.MethodGet, "/roles/nmnw", ScopeRolesRead, "/admin/api/roles/nmnw"},
		{http.MethodDelete, "/roles/per-client.v2", ScopeRolesDelete, "/admin/api/roles/per-client.v2"},
		{http.MethodPost, "/roles/nmnw/acls", ScopeRolesWrite, "/admin/api/roles/nmnw/acls"},
		{http.MethodPost, "/roles/nmnw/acls/remove", ScopeRolesWrite, "/admin/api/roles/nmnw/acls/remove"},
		{http.MethodPut, "/clients/gw1/roles/viewer", ScopeRolesWrite, "/admin/api/clients/gw1/roles/viewer"},
		{http.MethodDelete, "/clients/gw1/roles/viewer", ScopeRolesWrite, "/admin/api/clients/gw1/roles/viewer"},
		{http.MethodGet, "/docs/v1", ScopeServerRead, "/admin/api/docs/v1"},
		{http.MethodGet, "/docs/v12", ScopeServerRead, "/admin/api/docs/v12"},
		{http.MethodGet, "/clients", ScopeClientsRead, "/admin/api/clients"},
		{http.MethodPost, "/clients", ScopeClientsWrite, "/admin/api/clients"},
		{http.MethodGet, "/clients/nmnw-gateway-aabbccddeeff", ScopeClientsRead, "/admin/api/clients/nmnw-gateway-aabbccddeeff"},
		{http.MethodDelete, "/clients/gw.1_a", ScopeClientsDelete, "/admin/api/clients/gw.1_a"},
		{http.MethodPost, "/clients/gw1/disable", ScopeClientsWrite, "/admin/api/clients/gw1/disable"},
		{http.MethodPost, "/clients/gw1/enable", ScopeClientsWrite, "/admin/api/clients/gw1/enable"},
		{http.MethodPost, "/clients/gw1/password", ScopeCredentialsRotate, "/admin/api/clients/gw1/password"},
		{http.MethodGet, "/clients/", ScopeClientsRead, "/admin/api/clients"},
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
		{http.MethodGet, "/health"},
		{http.MethodPost, "/server"},
		{http.MethodPost, "/stats"},
		{http.MethodPut, "/clients"},
		{http.MethodDelete, "/clients"},
		{http.MethodPut, "/clients/gw1"},
		{http.MethodPatch, "/clients/gw1"},
		{http.MethodGet, "/clients/gw1/password"},
		{http.MethodGet, "/clients/gw1/disable"},
		{http.MethodPost, "/clients/gw1/roles"},
		{http.MethodPost, "/clients/gw1/disable/extra"},
		{http.MethodGet, "/clients/-bad"},
		{http.MethodGet, "/clients/has space"},
		{http.MethodGet, "/clients/has%2Fslash"},
		{http.MethodGet, "/clients/.."},
		{http.MethodGet, "/../clients"},
		{http.MethodGet, "/clients//gw1"},
		{http.MethodGet, `/clients\gw1`},
		{http.MethodGet, "/dynsec"},
		{http.MethodPost, "/docs"},
		{http.MethodPut, "/roles"},
		{http.MethodDelete, "/roles"},
		{http.MethodPut, "/roles/nmnw"},
		{http.MethodPatch, "/roles/nmnw"},
		{http.MethodGet, "/roles/nmnw/acls"},
		{http.MethodDelete, "/roles/nmnw/acls"},
		{http.MethodPut, "/roles/nmnw/acls"},
		{http.MethodPost, "/roles/nmnw/acls/add"},
		{http.MethodGet, "/roles/nmnw/acls/remove"},
		{http.MethodPost, "/roles/nmnw/acls/remove/extra"},
		{http.MethodPost, "/roles/nmnw/other"},
		{http.MethodGet, "/roles/-bad"},
		{http.MethodGet, "/roles/has space"},
		{http.MethodGet, "/roles/../clients"},
		{http.MethodGet, "/roles/" + "a" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{http.MethodPut, "/clients/gw1/roles"},
		{http.MethodPut, "/clients/gw1/roles/"},
		{http.MethodPost, "/clients/gw1/roles/viewer"},
		{http.MethodGet, "/clients/gw1/roles/viewer"},
		{http.MethodPatch, "/clients/gw1/roles/viewer"},
		{http.MethodPut, "/clients/gw1/roles/-bad"},
		{http.MethodPut, "/clients/gw1/roles/viewer/extra"},
		{http.MethodPut, "/clients/-bad/roles/viewer"},
		{http.MethodDelete, "/docs/v1"},
		{http.MethodGet, "/docs/latest"},
		{http.MethodGet, "/docs/v1/extra"},
		{http.MethodGet, "/docs/V1"},
		{http.MethodGet, "/docs/v1234"},
		{http.MethodGet, "/docs/../clients"},
		{http.MethodPost, "/commands"},
	}

	for _, tt := range tests {
		if _, err := Resolve(tt.method, tt.path); err == nil {
			t.Fatalf("Resolve(%s, %s) unexpectedly succeeded", tt.method, tt.path)
		}
	}
}

func TestEveryScopeIsPublishedByTheService(t *testing.T) {
	// Keeps the facade aligned with the scopes registered for the mw-mqtt resource.
	want := map[string]bool{
		"mqtt.clients.read": true, "mqtt.clients.write": true, "mqtt.clients.delete": true,
		"mqtt.credentials.rotate": true, "mqtt.roles.read": true, "mqtt.server.read": true,
	}
	for _, scope := range []string{ScopeClientsRead, ScopeClientsWrite, ScopeClientsDelete,
		ScopeCredentialsRotate, ScopeRolesRead, ScopeServerRead} {
		if !want[scope] {
			t.Fatalf("unexpected scope %q", scope)
		}
		delete(want, scope)
	}
	if len(want) != 0 {
		t.Fatalf("scopes not covered: %v", want)
	}
}

func TestRoleRoutesNeedTheRolesScopes(t *testing.T) {
	// Access-changing routes must never be satisfied by a clients scope.
	for _, tc := range []struct{ method, path, want string }{
		{http.MethodPut, "/clients/gw1/roles/viewer", ScopeRolesWrite},
		{http.MethodDelete, "/clients/gw1/roles/viewer", ScopeRolesWrite},
		{http.MethodPost, "/roles", ScopeRolesWrite},
		{http.MethodDelete, "/roles/viewer", ScopeRolesDelete},
	} {
		route, err := Resolve(tc.method, tc.path)
		if err != nil || route.Scope != tc.want {
			t.Fatalf("%s %s: %v %v, want scope %s", tc.method, tc.path, route, err, tc.want)
		}
		if route.Scope == ScopeClientsWrite || route.Scope == ScopeClientsDelete {
			t.Fatalf("%s %s must not be satisfied by a clients scope", tc.method, tc.path)
		}
	}
}
