package env

import (
	"os"
)

// GetEnvVar retrieves an environment variable; an unset variable returns an empty string
func GetEnvVar(name string) string {
	return os.Getenv(name)
}

// SetEnvVar sets an environment variable
func SetEnvVar(name, value string) error {
	return os.Setenv(name, value)
}
