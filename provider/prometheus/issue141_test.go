package prometheus

import (
	"testing"

	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/stretchr/testify/assert"
)

// Issue #141 claim 3: Config.Validate reports an out-of-range port with the httpserver sentinel
func TestIssue141_ConfigValidateInvalidPortSentinel(t *testing.T) {
	for name, port := range map[string]int{"negative": -1, "too high": 70000} {
		t.Run(name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.Port = port
			assert.ErrorIs(t, cfg.Validate(), httpserver.ErrInvalidPort)
		})
	}
}
