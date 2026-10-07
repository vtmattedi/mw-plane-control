package mwmqtt

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
)

// Audience is the MW Identity resource application for the MQTT admin API.
const Audience = "mw-mqtt"

const (
	ScopeClientsRead       = "mqtt.clients.read"
	ScopeClientsWrite      = "mqtt.clients.write"
	ScopeClientsDelete     = "mqtt.clients.delete"
	ScopeCredentialsRotate = "mqtt.credentials.rotate"
	ScopeRolesRead         = "mqtt.roles.read"
	ScopeServerRead        = "mqtt.server.read"
)

var ErrRouteNotFound = errors.New("MW MQTT admin route not found")

// docVersion mirrors the admin API's published version names (v1, v2, ...).
var docVersion = regexp.MustCompile(`^v[0-9]{1,3}$`)

// username mirrors the broker admin API's own username rule.
var username = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Route struct {
	DownstreamPath string
	Scope          string
}

// Resolve maps a facade request to exactly one downstream admin route and the single
// delegated scope it needs. Anything not listed here is not forwarded.
func Resolve(method string, facadePath string) (Route, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	path := cleanFacadePath(facadePath)
	if path == "" {
		return Route{}, ErrRouteNotFound
	}

	switch {
	case method == http.MethodGet && path == "/server":
		return route(path, ScopeServerRead), nil
	case method == http.MethodGet && path == "/stats":
		return route(path, ScopeServerRead), nil
	case method == http.MethodGet && path == "/docs":
		return route(path, ScopeServerRead), nil
	case method == http.MethodGet && path == "/roles":
		return route(path, ScopeRolesRead), nil
	case method == http.MethodGet && path == "/clients":
		return route(path, ScopeClientsRead), nil
	case method == http.MethodPost && path == "/clients":
		return route(path, ScopeClientsWrite), nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 && parts[0] == "docs" && method == http.MethodGet && docVersion.MatchString(parts[1]) {
		return route(path, ScopeServerRead), nil
	}
	if len(parts) < 2 || parts[0] != "clients" || !username.MatchString(parts[1]) {
		return Route{}, ErrRouteNotFound
	}

	switch {
	case len(parts) == 2 && method == http.MethodGet:
		return route(path, ScopeClientsRead), nil
	case len(parts) == 2 && method == http.MethodDelete:
		return route(path, ScopeClientsDelete), nil
	case len(parts) == 3 && parts[2] == "disable" && method == http.MethodPost:
		return route(path, ScopeClientsWrite), nil
	case len(parts) == 3 && parts[2] == "enable" && method == http.MethodPost:
		return route(path, ScopeClientsWrite), nil
	case len(parts) == 3 && parts[2] == "password" && method == http.MethodPost:
		return route(path, ScopeCredentialsRotate), nil
	default:
		return Route{}, ErrRouteNotFound
	}
}

func route(path string, scope string) Route {
	return Route{DownstreamPath: "/admin/api" + path, Scope: scope}
}

func cleanFacadePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if strings.Contains(value, "\\") ||
		strings.Contains(value, "//") ||
		strings.Contains(value, "..") ||
		len(value) > 512 {
		return ""
	}
	return strings.TrimSuffix(value, "/")
}
