//go:build integration

// Package integration runs end-to-end checks against a freshly built wafi
// binary. Run with: go test -tags integration ./integration/
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles wafi into a temp dir and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "wafi")

	// Resolve repo root from the test's working dir (integration/).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)

	cmd := exec.Command("go", "build", "-o", out, "./cmd/wafi")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, output)
	}
	return out
}

func TestBinary_Version(t *testing.T) {
	bin := buildBinary(t)

	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("wafi version: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "0.1.0") {
		t.Fatalf("version missing 0.1.0: %s", out)
	}
}

func TestBinary_RunEcho(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "run", "echo", "hello-wafi")
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wafi run echo: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "hello-wafi") {
		t.Fatalf("output missing echo: %s", out)
	}
}

func TestBinary_RunGitStatusShorterThanRaw(t *testing.T) {
	bin := buildBinary(t)

	// Need a real git repo with at least one commit; "No commits yet" header
	// falls outside the filter's grammar and triggers passthrough.
	repo := t.TempDir()
	mustRun(t, repo, "git", "init", "-q")
	mustRun(t, repo, "git", "config", "user.email", "test@wafi.local")
	mustRun(t, repo, "git", "config", "user.name", "wafi")
	mustRun(t, repo, "git", "commit", "--allow-empty", "-m", "init", "-q")
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		path := filepath.Join(repo, name)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	rawBytes, err := runIn(repo, "git", "status")
	if err != nil {
		t.Fatalf("raw git status: %v", err)
	}

	cmd := exec.Command(bin, "run", "git", "status")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+t.TempDir())
	filteredBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wafi run git status: %v\n%s", err, filteredBytes)
	}
	if len(filteredBytes) >= len(rawBytes) {
		t.Fatalf("filtered (%d bytes) should be shorter than raw (%d bytes)\nraw:\n%s\nfiltered:\n%s",
			len(filteredBytes), len(rawBytes), rawBytes, filteredBytes)
	}
}

func TestBinary_Doctor_NoPanic(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "doctor")
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+t.TempDir())
	// Doctor may exit 0 or 1 depending on environment; either is acceptable
	// as long as it doesn't panic or hang.
	out, _ := cmd.CombinedOutput()
	if strings.Contains(string(out), "panic:") {
		t.Fatalf("wafi doctor panicked:\n%s", out)
	}
	if !strings.Contains(string(out), "stash directory") {
		t.Fatalf("doctor output missing expected line:\n%s", out)
	}
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func runIn(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
