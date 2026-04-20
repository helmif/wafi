// Package ledger tracks token savings across wafi-filtered commands.
// Token counts are estimates using the ~4 chars/token heuristic (tiktoken
// average for English + code). These are approximations, not exact counts.
//
// Security invariants:
//   - Ledger file is written with 0600 (owner read/write only)
//   - Ledger dir is created with 0700
//   - Stored in XDG_STATE_HOME (or ~/.local/state) — not in project dir
//   - No command content or env vars are logged
package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	version     = 1
	maxSessions = 90
	// charsPerToken is the chars-per-token estimate (tiktoken avg for English+code).
	charsPerToken = 4
)

// EstimateTokens returns an approximate token count for b bytes of output.
func EstimateTokens(b int) int {
	if b <= 0 {
		return 0
	}
	return (b + charsPerToken - 1) / charsPerToken
}

// FilterStat holds per-filter aggregate statistics.
type FilterStat struct {
	Name        string
	CallCount   int64
	RawAvg      float64
	FilteredAvg float64
	SavedAvg    float64
}

// filterAccum is the mutable accumulator kept in memory (not serialised).
type filterAccum struct {
	callCount   int64
	rawTotal    int64
	filteredTotal int64
}

// LifetimeStats holds cumulative totals.
type LifetimeStats struct {
	CommandsFiltered   int64 `json:"commands_filtered"`
	CommandsPassthrough int64 `json:"commands_passthrough"`
	TokensRaw          int64 `json:"tokens_raw"`
	TokensFiltered     int64 `json:"tokens_filtered"`
	TokensSaved        int64 `json:"tokens_saved"`
	RepeatReadsBlocked int64 `json:"repeat_reads_blocked"`
}

// SessionEntry is one entry in the sessions list.
type SessionEntry struct {
	ID          string    `json:"id"`
	Started     time.Time `json:"started"`
	Commands    int64     `json:"commands"`
	TokensSaved int64     `json:"tokens_saved"`
}

// diskLedger is the on-disk JSON structure.
type diskLedger struct {
	Version  int           `json:"version"`
	Lifetime LifetimeStats `json:"lifetime"`
	Sessions []SessionEntry `json:"sessions"`
}

// Ledger is the in-memory representation with a live session.
type Ledger struct {
	path      string
	disk      diskLedger
	sessionID string
	sessionIdx int // index of current session in disk.Sessions (-1 if new)
	filterMap  map[string]*filterAccum
}

// Load reads the ledger from disk, creating it if absent. If the file is
// corrupt, it logs a warning to stderr, deletes the file, and starts fresh.
func Load(sessionID string) (*Ledger, error) {
	path, err := filePath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("ledger: create dir: %w", err)
	}

	dl, err := readDisk(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[wafi] ledger: corrupt file, resetting (%v)\n", err)
		_ = os.Remove(path)
		dl = diskLedger{Version: version}
	}
	if dl.Version == 0 {
		dl.Version = version
	}

	l := &Ledger{
		path:       path,
		disk:       dl,
		sessionID:  sessionID,
		sessionIdx: -1,
		filterMap:  make(map[string]*filterAccum),
	}
	l.ensureSession()
	if err := l.Save(); err != nil {
		return nil, err
	}
	return l, nil
}

func readDisk(path string) (diskLedger, error) {
	// #nosec G304 -- path is always the ledger file under the fixed XDG state
	// dir; never user-supplied.
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return diskLedger{Version: version}, nil
	}
	if err != nil {
		return diskLedger{}, err
	}
	var dl diskLedger
	if err := json.Unmarshal(data, &dl); err != nil {
		return diskLedger{}, err
	}
	return dl, nil
}

// ensureSession finds or creates the current session entry.
func (l *Ledger) ensureSession() {
	for i, s := range l.disk.Sessions {
		if s.ID == l.sessionID {
			l.sessionIdx = i
			return
		}
	}
	// New session — append and cap.
	entry := SessionEntry{
		ID:      l.sessionID,
		Started: time.Now().UTC(),
	}
	l.disk.Sessions = append(l.disk.Sessions, entry)
	if len(l.disk.Sessions) > maxSessions {
		l.disk.Sessions = l.disk.Sessions[len(l.disk.Sessions)-maxSessions:]
	}
	l.sessionIdx = len(l.disk.Sessions) - 1
}

// RecordCommand updates lifetime and session stats for one filtered or
// passthrough command. filterName is the filter that matched (empty string
// means passthrough). raw and filtered are byte lengths of the output.
func (l *Ledger) RecordCommand(filterName string, raw, filtered int, passthrough bool) error {
	rawTok := int64(EstimateTokens(raw))
	filtTok := int64(EstimateTokens(filtered))
	saved := max(rawTok-filtTok, 0)

	lt := &l.disk.Lifetime
	if passthrough {
		lt.CommandsPassthrough++
	} else {
		lt.CommandsFiltered++
	}
	lt.TokensRaw += rawTok
	lt.TokensFiltered += filtTok
	lt.TokensSaved += saved

	sess := &l.disk.Sessions[l.sessionIdx]
	sess.Commands++
	sess.TokensSaved += saved

	if filterName != "" {
		acc, ok := l.filterMap[filterName]
		if !ok {
			acc = &filterAccum{}
			l.filterMap[filterName] = acc
		}
		acc.callCount++
		acc.rawTotal += int64(raw)
		acc.filteredTotal += int64(filtered)
	}

	return l.Save()
}

// RecordRepeatBlocked increments the repeat-read-blocked counter.
func (l *Ledger) RecordRepeatBlocked() error {
	l.disk.Lifetime.RepeatReadsBlocked++
	return l.Save()
}

// CurrentSession returns a copy of the active session entry.
func (l *Ledger) CurrentSession() SessionEntry {
	return l.disk.Sessions[l.sessionIdx]
}

// Lifetime returns a copy of the lifetime stats.
func (l *Ledger) Lifetime() LifetimeStats {
	return l.disk.Lifetime
}

// FilterStats returns per-filter aggregate statistics computed from the
// in-memory accumulators (not persisted across process restarts).
func (l *Ledger) FilterStats() map[string]FilterStat {
	out := make(map[string]FilterStat, len(l.filterMap))
	for name, acc := range l.filterMap {
		if acc.callCount == 0 {
			continue
		}
		n := float64(acc.callCount)
		rawAvgBytes := float64(acc.rawTotal) / n
		filtAvgBytes := float64(acc.filteredTotal) / n
		out[name] = FilterStat{
			Name:        name,
			CallCount:   acc.callCount,
			RawAvg:      rawAvgBytes / charsPerToken,
			FilteredAvg: filtAvgBytes / charsPerToken,
			SavedAvg:    (rawAvgBytes - filtAvgBytes) / charsPerToken,
		}
	}
	return out
}

// Save atomically writes the ledger to disk (write .tmp → rename).
func (l *Ledger) Save() error {
	data, err := json.MarshalIndent(l.disk, "", "  ")
	if err != nil {
		return fmt.Errorf("ledger: marshal: %w", err)
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("ledger: write tmp: %w", err)
	}
	if err := os.Rename(tmp, l.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ledger: rename: %w", err)
	}
	return nil
}

// filePath returns the ledger path, respecting XDG_STATE_HOME.
func filePath() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "wafi", "ledger.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "wafi", "ledger.json"), nil
}
