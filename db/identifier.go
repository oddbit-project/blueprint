package db

import (
	"strings"

	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrInvalidIdentifier = utils.Error("invalid identifier")
)

// ValidIdentifier returns false if name contains characters that can break out of a quoted identifier:
// double quote, backslash (an escape character in ClickHouse identifiers) or NUL
func ValidIdentifier(name string) bool {
	return !strings.ContainsAny(name, "\"\\\x00")
}
