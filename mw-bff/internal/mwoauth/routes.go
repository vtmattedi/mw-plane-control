package mwoauth

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const Audience = "mw-oauth"

const (
	ScopeProjectsRead      = "oauth.projects.read"
	ScopeProjectsWrite     = "oauth.projects.write"
	ScopeCredentialsRotate = "oauth.credentials.rotate"
	ScopePolicyRead        = "oauth.policy.read"
	ScopePolicyWrite       = "oauth.policy.write"
	ScopeAuditRead         = "oauth.audit.read"
	ScopeMetricsRead       = "oauth.metrics.read"
	ScopeServerRead        = "oauth.server.read"
)

var ErrRouteNotFound = errors.New("MW OAuth admin route not found")

var safeSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Route struct {
	DownstreamPath string
	Scope          string
}

func Resolve(method string, facadePath string) (Route, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	path := cleanFacadePath(facadePath)
	if path == "" {
		return Route{}, ErrRouteNotFound
	}

	switch {
	case method == http.MethodGet && path == "/projects":
		return route(path, ScopeProjectsRead), nil
	case method == http.MethodPost && path == "/projects":
		return route(path, ScopeProjectsWrite), nil
	case method == http.MethodGet && path == "/audit":
		return route(path, ScopeAuditRead), nil
	case method == http.MethodGet && path == "/metrics":
		return route(path, ScopeMetricsRead), nil
	case method == http.MethodGet && path == "/server":
		return route(path, ScopeServerRead), nil
	case method == http.MethodGet && path == "/docs":
		return route(path, ScopeServerRead), nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return Route{}, ErrRouteNotFound
	}

	if len(parts) == 2 && parts[0] == "docs" && method == http.MethodGet && validSegment(parts[1]) {
		return route(path, ScopeServerRead), nil
	}
	if len(parts) == 2 && parts[0] == "audit" && method == http.MethodGet && validSegment(parts[1]) {
		return route(path, ScopeAuditRead), nil
	}

	if parts[0] != "projects" || len(parts) < 2 || !positiveDecimal(parts[1]) {
		return Route{}, ErrRouteNotFound
	}

	switch {
	case len(parts) == 2 && method == http.MethodPatch:
		return route(path, ScopeProjectsWrite), nil
	case len(parts) == 3 && method == http.MethodPost && parts[2] == "rotate-key":
		return route(path, ScopeCredentialsRotate), nil
	case len(parts) == 3 && method == http.MethodPost && parts[2] == "rotate-callback-secret":
		return route(path, ScopeCredentialsRotate), nil
	case len(parts) == 4 && parts[2] == "oauth" && parts[3] == "google" && method == http.MethodGet:
		return route(path, ScopePolicyRead), nil
	case len(parts) == 4 && parts[2] == "oauth" && parts[3] == "google" && method == http.MethodPut:
		return route(path, ScopePolicyWrite), nil
	default:
		return Route{}, ErrRouteNotFound
	}
}

func route(path string, scope string) Route {
	return Route{
		DownstreamPath: "/admin/api" + path,
		Scope:          scope,
	}
}

func cleanFacadePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if strings.Contains(value, "\\") || strings.Contains(value, "//") || strings.Contains(value, "..") {
		return ""
	}
	if len(value) > 512 {
		return ""
	}
	return strings.TrimSuffix(value, "/")
}

func positiveDecimal(value string) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}

func validSegment(value string) bool {
	return value != "" && len(value) <= 128 && safeSegment.MatchString(value)
}
