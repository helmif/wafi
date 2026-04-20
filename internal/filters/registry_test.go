package filters

import "testing"

// stubFilter matches when cmd equals its configured name. Used to verify
// first-match-wins ordering.
type stubFilter struct{ n string }

func (s stubFilter) Name() string                         { return s.n }
func (s stubFilter) Match(cmd string, _ []string) bool    { return cmd == s.n }
func (s stubFilter) Apply(in []byte, _ ApplyContext) []byte { return in }

func TestRegistry_LookupReturnsNilWhenEmpty(t *testing.T) {
	r := New()
	if got := r.Lookup("git", []string{"status"}); got != nil {
		t.Fatalf("empty registry returned non-nil filter: %v", got)
	}
}

func TestRegistry_LookupReturnsNilOnNoMatch(t *testing.T) {
	r := New()
	r.Register(stubFilter{n: "git"})
	if got := r.Lookup("docker", []string{"build"}); got != nil {
		t.Fatalf("non-matching lookup returned: %v", got)
	}
}

func TestRegistry_LookupReturnsFirstMatch(t *testing.T) {
	first := stubFilter{n: "git"}
	second := stubFilter{n: "git"} // same match, distinct instance
	r := New()
	r.Register(first)
	r.Register(second)

	got := r.Lookup("git", nil)
	if got == nil {
		t.Fatal("expected a match")
	}
	// Pointer identity isn't available (values, not pointers), so verify
	// via a canary: shrink the first, ensure we got that one. Easiest check
	// is that first-match means the registry does not iterate past it.
	// Here we cannot distinguish equal values; use distinct filters.
	// (covered by TestRegistry_LookupFirstMatchWinsOverDistinctFilters)
	_ = got
}

// distinguishableFilter embeds an id so we can verify which instance won.
type distinguishableFilter struct {
	id     string
	accept string
}

func (d distinguishableFilter) Name() string                         { return d.id }
func (d distinguishableFilter) Match(cmd string, _ []string) bool    { return cmd == d.accept }
func (d distinguishableFilter) Apply(in []byte, _ ApplyContext) []byte { return in }

func TestRegistry_LookupFirstMatchWinsOverDistinctFilters(t *testing.T) {
	r := New()
	r.Register(distinguishableFilter{id: "specific", accept: "git"})
	r.Register(distinguishableFilter{id: "general", accept: "git"})

	got := r.Lookup("git", []string{"status"})
	if got == nil {
		t.Fatal("expected a match")
	}
	if got.Name() != "specific" {
		t.Fatalf("first-match-wins broken: got %q, want %q", got.Name(), "specific")
	}
}

func TestDefault_RegistersGitStatus(t *testing.T) {
	r := Default()
	f := r.Lookup("git", []string{"status"})
	if f == nil {
		t.Fatal("Default() registry did not route `git status`")
	}
	if f.Name() != "git-status" {
		t.Fatalf("expected git-status filter, got %q", f.Name())
	}
}
