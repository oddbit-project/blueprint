package config

import (
	"os"
	"path/filepath"
	"strings"
)

// StrOrFile attempts to identify a valid file path from value; if value starts with "/" or "./" it will attempt to
// read the file contents and return it instead; if the file contains an extra \n it will be trimmed.
// If no file is found (either value does not start with "/" or "./", or file does not exist), value is returned.
func StrOrFile(value string) string {
	if strings.HasPrefix(value, string(filepath.Separator)) || strings.HasPrefix(value, "."+string(filepath.Separator)) {
		if content, err := os.ReadFile(value); err == nil {
			return strings.TrimSuffix(string(content), "\n")
		}
	}
	return value
}
