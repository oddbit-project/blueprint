package blueprint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const containerHelperEnv = "BLUEPRINT_CONTAINER_HELPER"

// runContainerHelper re-executes the test binary to run a container scenario that exits the process
func runContainerHelper(t *testing.T, scenario string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestContainerHelperProcess$")
	cmd.Env = append(os.Environ(), containerHelperEnv+"="+scenario)
	out, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "helper process timed out: %s", out)

	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	return string(out), code
}

// TestContainerHelperProcess is not a real test; it runs container scenarios in a subprocess
func TestContainerHelperProcess(t *testing.T) {
	scenario := os.Getenv(containerHelperEnv)
	if scenario == "" {
		t.Skip("helper process only")
	}

	c := NewContainer(nil)
	switch scenario {
	case "context-cancel":
		RegisterDestructor(func() error {
			fmt.Println("destructor-ran")
			return nil
		})
		c.Run(func(a interface{}) error {
			a.(*Container).CancelCtx()
			return nil
		})

	case "signal":
		RegisterDestructor(func() error {
			fmt.Printf("ctx-err-at-destructor=%v\n", c.Context.Err())
			return nil
		})
		c.Run(func(a interface{}) error {
			return syscall.Kill(os.Getpid(), syscall.SIGTERM)
		})

	case "signal-reentrant":
		RegisterDestructor(func() error {
			RegisterDestructor(func() error { return nil })
			fmt.Println("destructor-ran")
			return nil
		})
		c.Run(func(a interface{}) error {
			return syscall.Kill(os.Getpid(), syscall.SIGTERM)
		})

	case "abort-after-shutdown":
		Shutdown(nil)
		c.AbortFatal(errors.New("abort-marker"))
	}
	fmt.Println("unexpected-return")
	os.Exit(3)
}

func TestContainer_Run_ContextCancelRunsDestructors(t *testing.T) {
	out, code := runContainerHelper(t, "context-cancel")
	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "destructor-ran")
}

func TestContainer_Run_SignalCancelsContextBeforeDestructors(t *testing.T) {
	out, code := runContainerHelper(t, "signal")
	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "ctx-err-at-destructor=context canceled")
}

func TestContainer_Run_SignalDestructorRegistersDestructor(t *testing.T) {
	out, code := runContainerHelper(t, "signal-reentrant")
	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "destructor-ran")
}

func TestContainer_AbortFatal_AfterShutdownLogsError(t *testing.T) {
	out, code := runContainerHelper(t, "abort-after-shutdown")
	assert.NotEqual(t, 0, code, out)
	assert.Contains(t, out, "abort-marker")
}
