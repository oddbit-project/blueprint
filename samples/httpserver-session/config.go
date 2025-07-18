package main

import (
	"errors"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
)

type Config struct {
	Api     *httpserver.ServerConfig `json:"api"`
	Session *session.Config          `json:"session"`
	Log     *log.Config              `json:"log"`
}

// NewConfig build default config options
func NewConfig() *Config {
	return &Config{
		Api:     httpserver.NewServerConfig(),
		Session: session.NewConfig(),
		Log:     log.NewDefaultConfig(),
	}
}

// Validate app config
func (c *Config) Validate() error {
	if c.Api == nil {
		return errors.New("api configuration is required")
	}
	if err := c.Api.Validate(); err != nil {
		return err
	}

	if c.Session == nil {
		return errors.New("session configuration is required")
	}
	if err := c.Session.Validate(); err != nil {
		return err
	}

	if c.Log == nil {
		return errors.New("log configuration is required")
	}

	if err := c.Log.Validate(); err != nil {
		return err
	}
	return nil
}
