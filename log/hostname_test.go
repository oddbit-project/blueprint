package log

import (
	"bytes"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hostnameTestConfig(include bool) *Config {
	cfg := NewDefaultConfig()
	cfg.Format = LogFmtJson
	cfg.Level = "debug"
	cfg.IncludeHostname = include
	return cfg
}

// logLine writes one line through l and returns it parsed
func logLine(t *testing.T, l *Logger) KV {
	buf := &bytes.Buffer{}
	l.WithOutput(buf).Info("hostname test")
	return parseLogOutput(t, buf)
}

func TestIssue97_IncludeHostnameAddsField(t *testing.T) {
	expected, err := os.Hostname()
	require.NoError(t, err)

	l, err := hostnameTestConfig(true).Logger()
	require.NoError(t, err)

	assert.Equal(t, expected, logLine(t, l)["hostname"])
}

func TestIssue97_IncludeHostnameOnDerivedLoggers(t *testing.T) {
	expected, err := os.Hostname()
	require.NoError(t, err)
	cfg := hostnameTestConfig(true)

	ml, err := cfg.ModuleLogger("mod")
	require.NoError(t, err)
	assert.Equal(t, expected, logLine(t, ml)["hostname"], "ModuleLogger")

	prevLogger, prevLevel, prevFormat := log.Logger, zerolog.GlobalLevel(), zerolog.TimeFieldFormat
	t.Cleanup(func() {
		log.Logger = prevLogger
		zerolog.SetGlobalLevel(prevLevel)
		zerolog.TimeFieldFormat = prevFormat
	})
	require.NoError(t, Configure(cfg))

	assert.Equal(t, expected, logLine(t, New("mod"))["hostname"], "New")
	assert.Equal(t, expected, logLine(t, NewWithComponent("mod", "comp"))["hostname"], "NewWithComponent")
}

func TestIssue97_ExcludeHostnameOmitsField(t *testing.T) {
	cfg := hostnameTestConfig(false)

	l, err := cfg.Logger()
	require.NoError(t, err)
	assert.NotContains(t, logLine(t, l), "hostname")

	ml, err := cfg.ModuleLogger("mod")
	require.NoError(t, err)
	assert.NotContains(t, logLine(t, ml), "hostname")
}

func TestIssue97_ExcludeHostnameOnGlobalDerivedLoggers(t *testing.T) {
	prevLogger, prevLevel, prevFormat := log.Logger, zerolog.GlobalLevel(), zerolog.TimeFieldFormat
	t.Cleanup(func() {
		log.Logger = prevLogger
		zerolog.SetGlobalLevel(prevLevel)
		zerolog.TimeFieldFormat = prevFormat
	})
	require.NoError(t, Configure(hostnameTestConfig(false)))

	assert.NotContains(t, logLine(t, New("mod")), "hostname", "New")
	assert.NotContains(t, logLine(t, NewWithComponent("mod", "comp")), "hostname", "NewWithComponent")
}
