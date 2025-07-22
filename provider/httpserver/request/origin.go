package request

import (
	"net/url"
	"slices"
)

// ValidOrigin returns true if origin is valid
func ValidOrigin(origin string, protocols []string) bool {
	// Special cases
	if origin == "null" || origin == "*" {
		return true
	}

	// Parse as URL
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// Origins must have scheme and host, no path/query/fragment
	if u.Scheme == "" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}

	// Only allow http/https
	if !slices.Contains(protocols, u.Scheme) {
		return false
	}

	return true
}
