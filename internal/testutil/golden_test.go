package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheckGolden_Match(t *testing.T) {
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.Mkdir("testdata", 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := []byte("hello\n")
	if err := os.WriteFile(filepath.Join("testdata", "case.golden.txt"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	// Should not fail.
	CheckGolden(t, "case.golden.txt", want)
}

func TestCheckGolden_UpdateFlag(t *testing.T) {
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	dir := t.TempDir()
	_ = os.Chdir(dir)
	_ = os.Mkdir("testdata", 0o700)

	orig := *Update
	*Update = true
	t.Cleanup(func() { *Update = orig })

	payload := []byte("generated\n")
	CheckGolden(t, "new.golden.txt", payload)

	got, err := os.ReadFile(filepath.Join("testdata", "new.golden.txt"))
	if err != nil {
		t.Fatalf("expected -update to write file: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("written content mismatch: %q", got)
	}
}

// TestCheckGolden_FailurePaths exercises the mismatch + missing-file branches
// by spawning the test binary as a subprocess and asserting it exits non-zero.
// Without this subprocess trick, CheckGolden would mark the parent test failed.
func TestCheckGolden_FailurePaths(t *testing.T) {
	if os.Getenv("WAFI_TESTUTIL_CHILD") == "mismatch" {
		dir, _ := os.MkdirTemp("", "wafi-tu-child")
		_ = os.Chdir(dir)
		_ = os.Mkdir("testdata", 0o700)
		_ = os.WriteFile(filepath.Join("testdata", "m.golden.txt"), []byte("expected"), 0o600)
		CheckGolden(t, "m.golden.txt", []byte("actual"))
		return
	}
	if os.Getenv("WAFI_TESTUTIL_CHILD") == "missing" {
		dir, _ := os.MkdirTemp("", "wafi-tu-child")
		_ = os.Chdir(dir)
		_ = os.Mkdir("testdata", 0o700)
		CheckGolden(t, "gone.golden.txt", []byte("x"))
		return
	}

	exe, err := os.Executable()
	if err != nil {
		t.Skip("executable path unavailable")
	}

	for _, mode := range []string{"mismatch", "missing"} {
		cmd := exec.Command(exe, "-test.run=TestCheckGolden_FailurePaths", "-test.v")
		cmd.Env = append(os.Environ(), "WAFI_TESTUTIL_CHILD="+mode)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("%s: expected non-zero exit; got success. output:\n%s", mode, out)
		}
	}
}
