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

// StrOrFileIfExists reads and returns the contents of value if it points to an existing regular file.
// If value is not an existing file, the original string is returned unchanged.
func StrOrFileIfExists(value string) string {
	info, err := os.Stat(value)
	if err != nil || info.IsDir() {
		return value
	}

	content, err := os.ReadFile(value)
	if err != nil {
		return value
	}

	return strings.TrimSuffix(string(content), "\n")
}
