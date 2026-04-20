package filters

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// recordingFilter captures the args it was called with so tests can verify
// ApplyContext is forwarded unchanged.
type recordingFilter struct {
	name    string
	out     []byte
	gotCtx  ApplyContext
	gotIn   []byte
	matches bool
}

func (r *recordingFilter) Name() string                          { return r.name }
func (r *recordingFilter) Match(cmd string, args []string) bool  { return r.matches }
func (r *recordingFilter) Apply(in []byte, ctx ApplyContext) []byte {
	r.gotIn = in
	r.gotCtx = ctx
	return r.out
}

// panickingFilter always panics. Used to verify SafeApply's recovery.
type panickingFilter struct {
	name   string
	reason any
}

func (p panickingFilter) Name() string                         { return p.name }
func (panickingFilter) Match(cmd string, args []string) bool   { return true }
func (p panickingFilter) Apply(_ []byte, _ ApplyContext) []byte { panic(p.reason) }

func TestSafeApply_ReturnsFilterOutputOnSuccess(t *testing.T) {
	f := &recordingFilter{name: "rec", out: []byte("compressed")}
	in := []byte("raw input")
	ctx := ApplyContext{Cmd: "git", Args: []string{"status"}, ExitCode: 0}

	got, err := SafeApply(f, in, ctx)
	if err != nil {
		t.Fatalf("SafeApply returned error on success: %v", err)
	}
	if !bytes.Equal(got, []byte("compressed")) {
		t.Fatalf("got %q, want %q", got, "compressed")
	}
}

func TestSafeApply_ForwardsContext(t *testing.T) {
	f := &recordingFilter{name: "rec"}
	in := []byte("in")
	ctx := ApplyContext{
		Cmd:      "git",
		Args:     []string{"status"},
		ExitCode: 2,
		Stderr:   []byte("some stderr"),
	}

	if _, err := SafeApply(f, in, ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(f.gotIn, in) {
		t.Fatalf("filter received %q, want %q", f.gotIn, in)
	}
	if f.gotCtx.Cmd != "git" || f.gotCtx.ExitCode != 2 {
		t.Fatalf("filter received wrong ctx: %+v", f.gotCtx)
	}
	if !bytes.Equal(f.gotCtx.Stderr, []byte("some stderr")) {
		t.Fatalf("filter received wrong stderr: %q", f.gotCtx.Stderr)
	}
}

func TestSafeApply_RecoversPanicAndReturnsOriginal(t *testing.T) {
	raw := []byte("original bytes the user will actually see")
	f := panickingFilter{name: "boom", reason: "nope"}

	got, err := SafeApply(f, raw, ApplyContext{})
	if err == nil {
		t.Fatal("expected non-nil error from panicking filter, got nil")
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("pass-through broken: got %q, want %q", got, raw)
	}
	// Error must identify the filter so callers can log/stash it.
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error %q does not name the filter", err.Error())
	}
}

func TestSafeApply_RecoversFromErrorPanic(t *testing.T) {
	// A filter may panic with an error value (e.g. runtime.Error). Verify
	// SafeApply handles that too, not just string panics.
	f := panickingFilter{name: "errpanic", reason: errors.New("inner")}
	raw := []byte("raw")

	got, err := SafeApply(f, raw, ApplyContext{})
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("got %q, want raw pass-through", got)
	}
}

func TestSafeApply_NilOutputFromFilterIsReturnedAsIs(t *testing.T) {
	// A filter that deliberately returns nil (e.g. empty compression) must
	// not be rewritten by SafeApply — only panics trigger pass-through.
	f := &recordingFilter{name: "rec", out: nil}
	got, err := SafeApply(f, []byte("raw"), ApplyContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("got %q, want nil", got)
	}
}
