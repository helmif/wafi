// Package stash persists raw command output to disk when a filter is applied
// and the command failed. This guarantees the AI can always recover the
// full output if the compressed version lacks detail.
//
// Security invariants:
//   - Files are written with 0600 (owner read/write only)
//   - Stash dir is created with 0700
//   - Filenames are sanitized; no user-supplied path components
//   - Stored in XDG_STATE_HOME (or ~/.local/state) — not in project dir
package stash

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Entry describes what was stashed. Path is safe to include in AI output.
type Entry struct {
	Path      string
	SizeBytes int
}

var safeName = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// Save writes stdout and stderr to a timestamped file under the stash dir.
// It returns a user-facing path and size. On any error, it returns a zero
// Entry and the error — callers should NOT fail the whole invocation if
// stash fails; they should just skip the "full output at..." hint.
func Save(commandName string, stdout, stderr []byte) (Entry, error) {
	dir, err := Dir()
	if err != nil {
		return Entry{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Entry{}, fmt.Errorf("create stash dir: %w", err)
	}

	safeCmd := safeName.ReplaceAllString(commandName, "_")
	if safeCmd == "" {
		safeCmd = "unknown"
	}
	if len(safeCmd) > 40 {
		safeCmd = safeCmd[:40]
	}

	filename := fmt.Sprintf("%d_%s.log", time.Now().Unix(), safeCmd)
	path := filepath.Join(dir, filename)

	var b strings.Builder
	b.WriteString("# wafi stash\n")
	b.WriteString("# command: ")
	b.WriteString(commandName)
	b.WriteString("\n# timestamp: ")
	b.WriteString(time.Now().UTC().Format(time.RFC3339))
	b.WriteString("\n\n--- STDOUT ---\n")
	b.Write(stdout)
	b.WriteString("\n--- STDERR ---\n")
	b.Write(stderr)

	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return Entry{}, fmt.Errorf("write stash: %w", err)
	}

	return Entry{Path: path, SizeBytes: b.Len()}, nil
}

// Dir returns the stash directory, respecting XDG_STATE_HOME.
func Dir() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "wafi", "stash"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "wafi", "stash"), nil
}

// CleanupOlderThan removes stash files older than age. Meant to be called
// occasionally (e.g., from a `wafi stash clean` subcommand). Not called
// automatically to avoid surprise data loss.
func CleanupOlderThan(age time.Duration) (int, error) {
	dir, err := Dir()
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-age)
	removed := 0
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}
