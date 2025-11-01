package backend

import (
	"errors"

	"github.com/oddbit-project/blueprint/provider/htpasswd"
)

type HtpasswdBackend struct {
	container *htpasswd.Container
}

type HtpasswdConfig struct {
	Keys map[string]string `json:"userKeys"`
}

func NewHtpasswdConfig() *HtpasswdConfig {
	return &HtpasswdConfig{
		Keys: make(map[string]string),
	}
}

func (c *HtpasswdConfig) Validate() error {
	if c.Keys == nil {
		return errors.New("missing user keys")
	}
	return nil
}

func (c *HtpasswdConfig) NewHtpasswdBackend() (*HtpasswdBackend, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return NewHtpasswdBackendFromMap(c.Keys)
}

func NewHtpasswdBackend(c *htpasswd.Container) *HtpasswdBackend {
	return &HtpasswdBackend{
		container: c,
	}
}

func NewHtpasswdBackendFromMap(m map[string]string) (*HtpasswdBackend, error) {
	container := htpasswd.NewContainer()
	for k, v := range m {
		if err := container.AddUser(k, v); err != nil {
			return nil, err
		}
	}
	return &HtpasswdBackend{
		container: container,
	}, nil
}

func (h *HtpasswdBackend) ValidateUser(userName string, secret string) (bool, error) {
	return h.container.VerifyUser(userName, secret)
}
