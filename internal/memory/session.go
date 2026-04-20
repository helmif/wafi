// Package memory tracks file reads per session to detect redundant AI reads.
//
// Security invariants:
//   - Session files written with 0600 (owner read/write only)
//   - Session dir created with 0700
//   - Session ID derived from env or stable process attributes; never from
//     user-supplied file path components
//   - Stored in XDG_STATE_HOME (or ~/.local/state) — not in project dir
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ReadRecord holds metadata about one file observed in this session.
type ReadRecord struct {
	FirstRead time.Time `json:"first_read"`
	ReadCount int       `json:"read_count"`
	FileHash  string    `json:"content_hash"`
	TokenEst  int       `json:"token_est"`
}

// Session tracks files read during a single AI invocation session.
type Session struct {
	ID      string                `json:"id"`
	Started time.Time             `json:"started"`
	Reads   map[string]ReadRecord `json:"reads"`

	path string // disk path; not serialised
}

// RecordRead records a read of path with the given content.
// It returns alreadySeen=true only when the file was read before AND the
// content hash is unchanged.
func (s *Session) RecordRead(path string, content []byte) (alreadySeen bool, rec ReadRecord, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, ReadRecord{}, fmt.Errorf("resolve path: %w", err)
	}

	hash := sha256sum(content)
	tokenEst := len(content) / 4 // rough estimate, not exact

	existing, seen := s.Reads[abs]
	switch {
	case !seen:
		rec = ReadRecord{
			FirstRead: time.Now().UTC(),
			ReadCount: 1,
			FileHash:  hash,
			TokenEst:  tokenEst,
		}
		s.Reads[abs] = rec
		return false, rec, s.flush()

	case existing.FileHash == hash:
		existing.ReadCount++
		s.Reads[abs] = existing
		return true, existing, s.flush()

	default: // seen but content changed
		existing.ReadCount++
		existing.FileHash = hash
		existing.TokenEst = tokenEst
		s.Reads[abs] = existing
		return false, existing, s.flush()
	}
}

// flush writes the session atomically to disk.
func (s *Session) flush() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write session tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename session: %w", err)
	}
	return nil
}

// Load loads or creates the session for the current process.
// Session ID resolution order:
//  1. $WAFI_SESSION_ID
//  2. $CLAUDE_SESSION_ID
//  3. hash of (ppid + tty + start time truncated to nearest minute)
func Load() (*Session, error) {
	id := sessionID()

	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	path := filepath.Join(dir, id+".json")

	// #nosec G304 -- `path` is composed from a sanitized id (sanitize()) under
	// a fixed XDG state dir; no user-supplied path component reaches here.
	data, err := os.ReadFile(path)
	if err == nil {
		var s Session
		if jsonErr := json.Unmarshal(data, &s); jsonErr == nil {
			s.path = path
			if s.Reads == nil {
				s.Reads = make(map[string]ReadRecord)
			}
			return &s, nil
		}
	}

	s := &Session{
		ID:      id,
		Started: time.Now().UTC(),
		Reads:   make(map[string]ReadRecord),
		path:    path,
	}
	return s, s.flush()
}

// Dir returns the sessions directory, respecting XDG_STATE_HOME.
func Dir() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "wafi", "sessions"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "wafi", "sessions"), nil
}

// sessionID derives a stable session identifier.
func sessionID() string {
	if v := os.Getenv("WAFI_SESSION_ID"); v != "" {
		return sanitize(v)
	}
	if v := os.Getenv("CLAUDE_SESSION_ID"); v != "" {
		return sanitize(v)
	}
	return fallbackID()
}

// fallbackID builds a session ID from ppid, tty, and minute-truncated time.
func fallbackID() string {
	ppid := strconv.Itoa(os.Getppid())
	tty := ttyName()
	minute := time.Now().Truncate(time.Minute).Unix()
	raw := fmt.Sprintf("%s|%s|%d", ppid, tty, minute)
	sum := sha256.Sum256([]byte(raw))
	return "fallback-" + hex.EncodeToString(sum[:8])
}

// ttyName returns the controlling tty path, or "notty" if unavailable.
func ttyName() string {
	for _, fd := range []uintptr{0, 1, 2} {
		if name, err := ttyNameFromFd(fd); err == nil {
			return name
		}
	}
	return "notty"
}

// unmarshalSession decodes JSON into s; used in tests and Load.
func unmarshalSession(data []byte, s *Session) error {
	return json.Unmarshal(data, s)
}

func sha256sum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// sanitize removes characters unsafe for filenames.
func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s) && i < 128; i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "unknown"
	}
	return string(out)
}
