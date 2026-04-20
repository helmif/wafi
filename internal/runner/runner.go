// Package runner executes subprocesses and captures output while preserving
// process semantics (exit code, signal handling, working directory, env).
//
// Safety invariants:
//   - Exit code is preserved exactly (Claude Code relies on it for decisions)
//   - SIGINT/SIGTERM forwarded to subprocess (user Ctrl+C must work)
//   - Output capped at maxOutputBytes; beyond that, stream pass-through
//   - No shell interpretation — args passed directly to exec.Command
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// maxOutputBytes caps in-memory buffering. Beyond this, we fall back to
// direct streaming (no filter applied). 10MB is ~2.5M tokens worst case
// which no sane filter would process anyway.
const maxOutputBytes = 10 * 1024 * 1024

// Result holds everything a filter (or caller) needs to know about a run.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	// Truncated indicates output exceeded maxOutputBytes and was streamed
	// directly instead of captured. Filters must not run on truncated output.
	Truncated bool
}

// Run executes name with args. It captures stdout/stderr up to maxOutputBytes
// and propagates the exit code. If the output exceeds the cap, it streams
// directly to os.Stdout/os.Stderr and returns Truncated=true.
//
// It forwards SIGINT and SIGTERM to the subprocess so user Ctrl+C behaves
// naturally. The returned Result always has a valid ExitCode even on error,
// so callers can `os.Exit(result.ExitCode)` safely.
func Run(ctx context.Context, name string, args []string) (*Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ() // inherit full env; we don't leak or filter

	var stdoutBuf, stderrBuf bytes.Buffer

	// Use LimitedWriter so buffering stops at the cap. We tee to os.Std* too
	// but only after we decide filter will run — so for now, just buffer.
	// If we hit the cap mid-stream, we flush buffered to real stdout and
	// switch to pass-through for the remainder.
	stdoutCap := &capWriter{buf: &stdoutBuf, limit: maxOutputBytes, passthrough: os.Stdout}
	stderrCap := &capWriter{buf: &stderrBuf, limit: maxOutputBytes, passthrough: os.Stderr}

	cmd.Stdout = stdoutCap
	cmd.Stderr = stderrCap

	// Forward signals: if user hits Ctrl+C, kill the subprocess gracefully.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if err := cmd.Start(); err != nil {
		return &Result{ExitCode: 127}, fmt.Errorf("start %q: %w", name, err)
	}

	// Handle signals in background.
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-sigCh:
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		case <-done:
			return
		}
	}()

	waitErr := cmd.Wait()
	close(done)

	result := &Result{
		Stdout:    stdoutBuf.Bytes(),
		Stderr:    stderrBuf.Bytes(),
		ExitCode:  exitCodeFrom(waitErr, cmd),
		Truncated: stdoutCap.overflowed || stderrCap.overflowed,
	}
	return result, nil
}

// exitCodeFrom extracts the real exit code whether the command exited
// normally, was killed by signal, or failed to start.
func exitCodeFrom(err error, cmd *exec.Cmd) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	// If cmd.ProcessState is available, use it.
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	// Unknown failure; 1 is conventional for "generic error".
	return 1
}

// capWriter buffers writes up to limit. If limit is exceeded, it flushes the
// buffer to passthrough and switches to passthrough-only mode.
type capWriter struct {
	buf         *bytes.Buffer
	limit       int
	passthrough io.Writer
	overflowed  bool
}

func (c *capWriter) Write(p []byte) (int, error) {
	if c.overflowed {
		return c.passthrough.Write(p)
	}
	remaining := c.limit - c.buf.Len()
	if len(p) <= remaining {
		return c.buf.Write(p)
	}
	// Overflow: flush buffered + remainder of p to passthrough.
	c.overflowed = true
	if _, err := c.passthrough.Write(c.buf.Bytes()); err != nil {
		return 0, err
	}
	c.buf.Reset()
	return c.passthrough.Write(p)
}
