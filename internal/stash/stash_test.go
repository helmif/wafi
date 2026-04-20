package stash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/wafi-test")
	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	want := "/tmp/wafi-test/wafi/stash"
	if got != want {
		t.Fatalf("Dir=%q, want %q", got, want)
	}
}

func TestDir_HomeFallback(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if !strings.Contains(got, ".local/state/wafi/stash") {
		t.Fatalf("Dir=%q missing expected suffix", got)
	}
}

func TestSave_WritesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	entry, err := Save("git", []byte("out"), []byte("err"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if entry.Path == "" {
		t.Fatal("entry.Path is empty")
	}
	if entry.SizeBytes == 0 {
		t.Fatal("SizeBytes is 0")
	}

	data, err := os.ReadFile(entry.Path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "--- STDOUT ---") || !strings.Contains(s, "out") {
		t.Fatalf("missing stdout content: %s", s)
	}
	if !strings.Contains(s, "--- STDERR ---") || !strings.Contains(s, "err") {
		t.Fatalf("missing stderr content: %s", s)
	}
	if !strings.Contains(s, "# command: git") {
		t.Fatalf("missing command header: %s", s)
	}

	info, err := os.Stat(entry.Path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm=%o, want 0600", info.Mode().Perm())
	}
}

func TestSave_SanitizesCommandName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	entry, err := Save("../evil/name with spaces", []byte("x"), nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	base := filepath.Base(entry.Path)
	if strings.Contains(base, "..") || strings.Contains(base, "/") || strings.Contains(base, " ") {
		t.Fatalf("filename not sanitized: %s", base)
	}
}

func TestSave_EmptyCommandFallsBackToUnknown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	entry, err := Save("", []byte("x"), nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.Contains(filepath.Base(entry.Path), "unknown") {
		t.Fatalf("expected unknown fallback, got %s", entry.Path)
	}
}

func TestSave_TruncatesLongName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	long := strings.Repeat("a", 100)
	entry, err := Save(long, []byte("x"), nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	base := filepath.Base(entry.Path)
	// filename is "<unix>_<name>.log"; ensure the "a" section <= 40 chars
	parts := strings.SplitN(strings.TrimSuffix(base, ".log"), "_", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected filename: %s", base)
	}
	if len(parts[1]) > 40 {
		t.Fatalf("name not truncated: %d chars", len(parts[1]))
	}
}

func TestCleanupOlderThan_MissingDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "nothing"))
	n, err := CleanupOlderThan(time.Hour)
	if err != nil {
		t.Fatalf("CleanupOlderThan: %v", err)
	}
	if n != 0 {
		t.Fatalf("n=%d, want 0", n)
	}
}

func TestCleanupOlderThan_RemovesOldFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	// Create one "old" and one "new" file.
	stashDir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if err := os.MkdirAll(stashDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	oldPath := filepath.Join(stashDir, "old.log")
	newPath := filepath.Join(stashDir, "new.log")
	if err := os.WriteFile(oldPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldPath, past, past); err != nil {
		t.Fatal(err)
	}

	n, err := CleanupOlderThan(24 * time.Hour)
	if err != nil {
		t.Fatalf("CleanupOlderThan: %v", err)
	}
	if n != 1 {
		t.Fatalf("n=%d, want 1", n)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("old file not removed")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatal("new file unexpectedly removed")
	}
}
