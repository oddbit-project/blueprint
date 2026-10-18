package blueprint

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/types/callstack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const issue145HelperEnv = "BLUEPRINT_ISSUE145_HELPER"

// TestIssue145HelperProcess is not a real test; it runs shutdown scenarios in a subprocess.
// It is separate from TestContainerHelperProcess so that container_test.go still builds at a core
// without ShutdownContext
func TestIssue145HelperProcess(t *testing.T) {
	scenario := os.Getenv(issue145HelperEnv)
	if scenario == "" {
		t.Skip("helper process only")
	}

	c := NewContainer(nil)
	switch scenario {
	case "sigterm-inflight":
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fmt.Println("listen-error", err)
			os.Exit(3)
		}
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("handler-started")
			time.Sleep(300 * time.Millisecond)
			_, _ = w.Write([]byte("ok"))
		})}
		RegisterDestructor(func() error {
			return srv.Shutdown(ShutdownContext())
		})
		c.Run(func(any) error {
			go func() { _ = srv.Serve(ln) }()
			fmt.Println("addr=" + ln.Addr().String())
			return nil
		})

	case "abort-fatal":
		RegisterDestructor(func() error {
			ctx := ShutdownContext()
			_, hasDeadline := ctx.Deadline()
			fmt.Printf("destructor-ctx deadline=%v err=%v\n", hasDeadline, ctx.Err())
			return nil
		})
		c.AbortFatal(errors.New("issue145-abort"))
	}
	fmt.Println("unexpected-return")
	os.Exit(3)
}

// resetShutdown145 gives a test a fresh destructor stack and shutdown state, restored afterwards
func resetShutdown145(t *testing.T) {
	t.Helper()
	shutdownMx.Lock()
	origDestructors, origTimeout, origCtx := appDestructors, shutdownTimeout, shutdownCtx
	appDestructors, shutdownCtx = callstack.NewCallStack(), nil
	shutdownMx.Unlock()
	t.Cleanup(func() {
		shutdownMx.Lock()
		appDestructors, shutdownTimeout, shutdownCtx = origDestructors, origTimeout, origCtx
		shutdownMx.Unlock()
	})
}

// claim 1: a request in flight at SIGTERM completes when the destructor uses ShutdownContext
func TestIssue145_SigtermDrainsInFlightRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestIssue145HelperProcess$")
	cmd.Env = append(os.Environ(), issue145HelperEnv+"=sigterm-inflight")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	sc := bufio.NewScanner(stdout)
	require.True(t, sc.Scan(), "helper printed nothing")
	line := sc.Text()
	require.True(t, strings.HasPrefix(line, "addr="), "unexpected helper output: %s", line)
	addr := strings.TrimPrefix(line, "addr=")

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for sc.Scan() {
			if sc.Text() == "handler-started" {
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
		}
	}()

	resp, err := http.Get("http://" + addr)
	require.NoError(t, err, "in-flight request was dropped on SIGTERM")
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
	<-readerDone // os/exec: all reads from the pipe must finish before Wait
	assert.NoError(t, cmd.Wait())
}

// claim 5: not cancelled at destructor start; one deadline, timeout from the start of Shutdown
func TestIssue145_ShutdownContextSharedDeadline(t *testing.T) {
	resetShutdown145(t)
	SetShutdownTimeout(2 * time.Second)

	var firstErr, secondErr error
	var firstDeadline, secondDeadline time.Time
	var firstOk, secondOk bool
	// destructors run in reverse order: this one runs second
	RegisterDestructor(func() error {
		ctx := ShutdownContext()
		secondErr = ctx.Err()
		secondDeadline, secondOk = ctx.Deadline()
		return nil
	})
	RegisterDestructor(func() error {
		ctx := ShutdownContext()
		firstErr = ctx.Err()
		firstDeadline, firstOk = ctx.Deadline()
		time.Sleep(20 * time.Millisecond)
		return nil
	})
	// runs first and never asks for the context, so a context created on first use would start late
	RegisterDestructor(func() error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	start := time.Now()
	Shutdown(nil)
	end := time.Now()

	assert.NoError(t, firstErr)
	assert.NoError(t, secondErr)
	require.True(t, firstOk, "first destructor's context has no deadline")
	require.True(t, secondOk, "second destructor's context has no deadline")
	assert.Equal(t, firstDeadline, secondDeadline, "destructors got different deadlines")
	assert.False(t, firstDeadline.Before(start.Add(2*time.Second)), "deadline %v earlier than start+timeout", firstDeadline)
	assert.False(t, firstDeadline.After(start.Add(2*time.Second+100*time.Millisecond)),
		"deadline %v not counted from the start of Shutdown (%v)", firstDeadline, start)
	assert.Less(t, end.Sub(start), 2*time.Second)
}

// guard: the context expires at the timeout, so a destructor waiting on it is bounded
func TestIssue145_ShutdownContextExpires(t *testing.T) {
	resetShutdown145(t)
	SetShutdownTimeout(50 * time.Millisecond)

	var waitErr error
	var waited time.Duration
	RegisterDestructor(func() error {
		begin := time.Now()
		select {
		case <-ShutdownContext().Done():
			waitErr = ShutdownContext().Err()
		case <-time.After(5 * time.Second):
		}
		waited = time.Since(begin)
		return nil
	})
	Shutdown(nil)

	assert.ErrorIs(t, waitErr, context.DeadlineExceeded)
	assert.Less(t, waited, 2*time.Second)
}

// guard: before Shutdown there is no deadline and nothing is cancelled
func TestIssue145_ShutdownContextBeforeShutdown(t *testing.T) {
	resetShutdown145(t)

	ctx := ShutdownContext()
	assert.NoError(t, ctx.Err())
	_, ok := ctx.Deadline()
	assert.False(t, ok)
}

// guard: after Shutdown returns, the shutdown context is cancelled (its timer is released)
func TestIssue145_ShutdownContextCancelledAfterShutdown(t *testing.T) {
	resetShutdown145(t)
	SetShutdownTimeout(time.Minute)

	Shutdown(nil)
	assert.ErrorIs(t, ShutdownContext().Err(), context.Canceled)
}

// guard: the AbortFatal path also hands destructors a live, bounded context
func TestIssue145_AbortFatalDestructorGetsBoundedContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestIssue145HelperProcess$")
	cmd.Env = append(os.Environ(), issue145HelperEnv+"=abort-fatal")
	out, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "helper process timed out: %s", out)
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "helper did not exit with an error: %s", out)
	assert.Equal(t, 1, exitErr.ExitCode(), out)
	assert.Contains(t, string(out), "destructor-ctx deadline=true err=<nil>")
}
