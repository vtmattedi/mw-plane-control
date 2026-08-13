package httpapi

import (
	"net/url"
	"strings"
)

func safeReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" ||
		!strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "//") ||
		strings.Contains(value, `\`) {
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil ||
		parsed.IsAbs() ||
		parsed.Host != "" ||
		!strings.HasPrefix(parsed.Path, "/") ||
		strings.HasPrefix(parsed.Path, "//") {
		return ""
	}

	return value
}
