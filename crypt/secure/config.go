package secure

import (
	"github.com/oddbit-project/blueprint/utils/env"
	"github.com/oddbit-project/blueprint/utils/fs"
	"strings"
)

// DefaultCredentialConfig misc options for credentials
// if different field names are required, just implement CredentialConfig interface
type DefaultCredentialConfig struct {
	Password       string `json:"password"`       // Password plaintext password; if set, is used instead of the rest
	PasswordEnvVar string `json:"passwordEnvVar"` // PasswordEnvVar name of env var with secret
	PasswordFile   string `json:"passwordFile"`   // PasswordFile name of secrets file, to be read; if none of the above set, this one is used
}

type KeyConfig struct {
	Key       string `json:"key"` // Key
	KeyEnvVar string `json:"keyEnvVar"`
	KeyFile   string `json:"keyFile"`
}

// IsEmpty returns true if credential source is empty
func (c DefaultCredentialConfig) IsEmpty() bool {
	return strings.TrimSpace(c.Password) == "" &&
		strings.TrimSpace(c.PasswordEnvVar) == "" &&
		strings.TrimSpace(c.PasswordFile) == ""
}

// Fetch retrieve the contents of the credential
func (c DefaultCredentialConfig) Fetch() (string, error) {
	plainText := strings.TrimSpace(c.Password)
	if plainText == "" {
		// attempt to read env var, if set
		envVar := strings.TrimSpace(c.PasswordEnvVar)
		if envVar == "" {
			// attempt to read secrets file, if set
			secretsFile := c.PasswordFile
			if secretsFile == "" {
				plainText = ""
			} else {
				// read secrets
				var err error
				if plainText, err = fs.ReadString(secretsFile); err != nil {
					return "", err
				}
			}
		} else {
			// read from env var and clear it
			plainText = env.GetEnvVar(envVar)
			_ = env.SetEnvVar(envVar, "")
		}
	}
	return plainText, nil
}

// IsEmpty returns true if credential source is empty
func (c KeyConfig) IsEmpty() bool {
	return strings.TrimSpace(c.KeyFile) == "" &&
		strings.TrimSpace(c.KeyEnvVar) == "" &&
		strings.TrimSpace(c.KeyFile) == ""
}

// Fetch retrieve the contents of the credential
func (c KeyConfig) Fetch() (string, error) {
	plainText := strings.TrimSpace(c.Key)
	if plainText == "" {
		// attempt to read env var, if set
		envVar := strings.TrimSpace(c.KeyEnvVar)
		if envVar == "" {
			// attempt to read secrets file, if set
			secretsFile := c.KeyFile
			if secretsFile == "" {
				plainText = ""
			} else {
				// read secrets
				var err error
				if plainText, err = fs.ReadString(secretsFile); err != nil {
					return "", err
				}
			}
		} else {
			// read from env var and clear it
			plainText = env.GetEnvVar(envVar)
			_ = env.SetEnvVar(envVar, "")
		}
	}
	return plainText, nil
}
