package mwmail

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const Audience = "mw-mail"

const (
	ScopeProjectsRead      = "mail.projects.read"
	ScopeProjectsWrite     = "mail.projects.write"
	ScopeCredentialsRotate = "mail.credentials.rotate"
	ScopeEmailsRead        = "mail.emails.read"
	ScopeUsageRead         = "mail.usage.read"
)

var ErrRouteNotFound = errors.New("MW Mail admin route not found")

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
	case method == http.MethodGet && path == "/health":
		return route(path, ScopeProjectsRead), nil
	case method == http.MethodGet && path == "/projects":
		return route(path, ScopeProjectsRead), nil
	case method == http.MethodPost && path == "/projects":
		return route(path, ScopeProjectsWrite), nil
	case method == http.MethodGet && path == "/emails":
		return route(path, ScopeEmailsRead), nil
	case method == http.MethodGet && path == "/emails/by-project":
		return route(path, ScopeUsageRead), nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 && parts[0] == "emails" &&
		method == http.MethodGet && positiveDecimal(parts[1]) {
		return route(path, ScopeEmailsRead), nil
	}

	if len(parts) < 2 || parts[0] != "projects" || !positiveDecimal(parts[1]) {
		return Route{}, ErrRouteNotFound
	}

	switch {
	case len(parts) == 2 && method == http.MethodPatch:
		return route(path, ScopeProjectsWrite), nil
	case len(parts) == 3 && parts[2] == "rotate-key" && method == http.MethodPost:
		return route(path, ScopeCredentialsRotate), nil
	case len(parts) == 3 && parts[2] == "request-domains" && method == http.MethodGet:
		return route(path, ScopeProjectsRead), nil
	case len(parts) == 3 && parts[2] == "request-domains" && method == http.MethodPut:
		return route(path, ScopeProjectsWrite), nil
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
	if strings.Contains(value, "\\") ||
		strings.Contains(value, "//") ||
		strings.Contains(value, "..") ||
		len(value) > 512 {
		return ""
	}
	return strings.TrimSuffix(value, "/")
}

func positiveDecimal(value string) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}
