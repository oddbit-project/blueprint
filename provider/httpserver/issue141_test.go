package httpserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var invalidPorts141 = map[string]int{"negative": -1, "too high": 65536, "far too high": 70000}

// Issue #141 claim 1: an out-of-range port is reported with the ErrInvalidPort sentinel
func TestIssue141_ValidateInvalidPortSentinel(t *testing.T) {
	for name, port := range invalidPorts141 {
		t.Run(name, func(t *testing.T) {
			cfg := NewServerConfig()
			cfg.Port = port
			assert.ErrorIs(t, cfg.Validate(), ErrInvalidPort)
		})
	}
}

// Issue #141 claim 1, through both constructors
func TestIssue141_NewServerInvalidPortSentinel(t *testing.T) {
	for name, port := range invalidPorts141 {
		t.Run(name, func(t *testing.T) {
			cfg := NewServerConfig()
			cfg.Port = port
			srv, err := NewServer(cfg, nil)
			assert.ErrorIs(t, err, ErrInvalidPort)
			assert.Nil(t, srv)

			srv, err = cfg.NewServer(nil)
			assert.ErrorIs(t, err, ErrInvalidPort)
			assert.Nil(t, srv)
		})
	}
}

// Issue #141 claim 2: the error text is unchanged
func TestIssue141_InvalidPortTextUnchanged(t *testing.T) {
	cfg := NewServerConfig()
	cfg.Port = -1
	assert.EqualError(t, cfg.Validate(), "port must be between 0 and 65535")
}

// Guard: ports inside the range are accepted, and 0 becomes the default port
func TestIssue141_GuardValidPortsAccepted(t *testing.T) {
	for name, port := range map[string]int{"lowest": 1, "highest": 65535, "zero": 0} {
		t.Run(name, func(t *testing.T) {
			cfg := NewServerConfig()
			cfg.Port = port
			assert.NoError(t, cfg.Validate())
			if port == 0 {
				assert.Equal(t, ServerDefaultPort, cfg.Port)
			} else {
				assert.Equal(t, port, cfg.Port)
			}
		})
	}
}
