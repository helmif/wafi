package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun_Success(t *testing.T) {
	r, err := Run(context.Background(), "echo", []string{"hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.ExitCode != 0 {
		t.Fatalf("exit=%d, want 0", r.ExitCode)
	}
	if got := strings.TrimSpace(string(r.Stdout)); got != "hello" {
		t.Fatalf("stdout=%q, want %q", got, "hello")
	}
	if r.Truncated {
		t.Fatal("unexpected truncation")
	}
}

func TestRun_NonZeroExit(t *testing.T) {
	r, err := Run(context.Background(), "sh", []string{"-c", "exit 7"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.ExitCode != 7 {
		t.Fatalf("exit=%d, want 7", r.ExitCode)
	}
}

func TestRun_StderrCaptured(t *testing.T) {
	r, err := Run(context.Background(), "sh", []string{"-c", "echo err 1>&2"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := strings.TrimSpace(string(r.Stderr)); got != "err" {
		t.Fatalf("stderr=%q, want %q", got, "err")
	}
}

func TestRun_NotFound(t *testing.T) {
	r, err := Run(context.Background(), "this-binary-does-not-exist-wafi-test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if r == nil || r.ExitCode != 127 {
		t.Fatalf("want exit=127, got result=%+v", r)
	}
}

func TestCapWriter_UnderLimit(t *testing.T) {
	var buf bytes.Buffer
	var pass bytes.Buffer
	c := &capWriter{buf: &buf, limit: 100, passthrough: &pass}
	n, err := c.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("write: n=%d err=%v", n, err)
	}
	if c.overflowed {
		t.Fatal("should not overflow")
	}
	if buf.String() != "hello" {
		t.Fatalf("buf=%q", buf.String())
	}
	if pass.Len() != 0 {
		t.Fatalf("pass should be empty, got %q", pass.String())
	}
}

func TestCapWriter_Overflow(t *testing.T) {
	var buf bytes.Buffer
	var pass bytes.Buffer
	c := &capWriter{buf: &buf, limit: 5, passthrough: &pass}

	if _, err := c.Write([]byte("abc")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	// Next write exceeds remaining (2) → overflow.
	if _, err := c.Write([]byte("123456")); err != nil {
		t.Fatalf("overflow write: %v", err)
	}
	if !c.overflowed {
		t.Fatal("expected overflow")
	}
	// Once overflowed, writes go straight to passthrough.
	if _, err := c.Write([]byte("xyz")); err != nil {
		t.Fatalf("post-overflow write: %v", err)
	}
	got := pass.String()
	if !strings.Contains(got, "abc") || !strings.Contains(got, "123456") || !strings.Contains(got, "xyz") {
		t.Fatalf("passthrough missing content: %q", got)
	}
}

func TestRun_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, _ := Run(ctx, "sh", []string{"-c", "sleep 5"})
	// Expect non-zero exit because the context cancelled before or during start.
	if r == nil {
		t.Fatal("nil result")
	}
	if r.ExitCode == 0 {
		t.Fatalf("expected non-zero exit on cancelled context, got %d", r.ExitCode)
	}
}
