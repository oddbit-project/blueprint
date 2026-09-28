package migrations

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestDefaultProgressFn_PercentInArgs(t *testing.T) {
	const name = "001_100%d_done.sql"
	tests := []struct {
		name     string
		msgType  int
		err      error
		expected string
	}{
		{"run", MsgRunMigration, nil, "Running migration '" + name + "'..."},
		{"finished", MsgFinishedMigration, nil, "Migration '" + name + "' finished successfully"},
		{"skip", MsgSkipMigration, nil, "Migration '" + name + "' already run, skipping"},
		{"error", MsgError, errors.New("50% failed"), "Error executing migration '" + name + "': 50% failed"},
		{"default", 0, nil, "Running migration '" + name + "'..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() { DefaultProgressFn(tt.msgType, name, tt.err) })
			assert.Contains(t, out, tt.expected)
			assert.NotContains(t, out, "%!")
		})
	}
}
