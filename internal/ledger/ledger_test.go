package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func setupTempLedger(t *testing.T, sessionID string) (*Ledger, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	l, err := Load(sessionID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return l, filepath.Join(dir, "wafi", "ledger.json")
}

func TestEstimateTokens(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, 0},
		{-1, 0},
		{4, 1},
		{5, 2},
		{100, 25},
		{101, 26},
	}
	for _, c := range cases {
		if got := EstimateTokens(c.in); got != c.want {
			t.Errorf("EstimateTokens(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestLoadCreatesFile(t *testing.T) {
	_, path := setupTempLedger(t, "sess-1")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected ledger file at %s: %v", path, err)
	}
	fi, _ := os.Stat(path)
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 0600", perm)
	}
}

func TestRecordCommandFiltered(t *testing.T) {
	l, _ := setupTempLedger(t, "sess-1")

	if err := l.RecordCommand("git_status", 400, 100, false); err != nil {
		t.Fatal(err)
	}

	lt := l.Lifetime()
	if lt.CommandsFiltered != 1 {
		t.Errorf("CommandsFiltered = %d, want 1", lt.CommandsFiltered)
	}
	if lt.CommandsPassthrough != 0 {
		t.Errorf("CommandsPassthrough = %d, want 0", lt.CommandsPassthrough)
	}
	// 400 bytes → 100 tokens raw; 100 bytes → 25 tokens filtered; saved = 75
	if lt.TokensRaw != 100 {
		t.Errorf("TokensRaw = %d, want 100", lt.TokensRaw)
	}
	if lt.TokensFiltered != 25 {
		t.Errorf("TokensFiltered = %d, want 25", lt.TokensFiltered)
	}
	if lt.TokensSaved != 75 {
		t.Errorf("TokensSaved = %d, want 75", lt.TokensSaved)
	}

	sess := l.CurrentSession()
	if sess.Commands != 1 {
		t.Errorf("sess.Commands = %d, want 1", sess.Commands)
	}
	if sess.TokensSaved != 75 {
		t.Errorf("sess.TokensSaved = %d, want 75", sess.TokensSaved)
	}
}

func TestRecordCommandPassthrough(t *testing.T) {
	l, _ := setupTempLedger(t, "sess-2")

	if err := l.RecordCommand("", 200, 200, true); err != nil {
		t.Fatal(err)
	}

	lt := l.Lifetime()
	if lt.CommandsPassthrough != 1 {
		t.Errorf("CommandsPassthrough = %d, want 1", lt.CommandsPassthrough)
	}
	if lt.TokensSaved != 0 {
		t.Errorf("TokensSaved = %d, want 0", lt.TokensSaved)
	}
}

func TestRecordRepeatBlocked(t *testing.T) {
	l, _ := setupTempLedger(t, "sess-3")

	_ = l.RecordRepeatBlocked()
	_ = l.RecordRepeatBlocked()

	lt := l.Lifetime()
	if lt.RepeatReadsBlocked != 2 {
		t.Errorf("RepeatReadsBlocked = %d, want 2", lt.RepeatReadsBlocked)
	}
}

func TestPersistenceAcrossLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	l1, err := Load("sess-persist")
	if err != nil {
		t.Fatal(err)
	}
	_ = l1.RecordCommand("git_status", 800, 200, false)

	l2, err := Load("sess-persist")
	if err != nil {
		t.Fatal(err)
	}
	lt := l2.Lifetime()
	if lt.CommandsFiltered != 1 {
		t.Errorf("after reload CommandsFiltered = %d, want 1", lt.CommandsFiltered)
	}
	if lt.TokensSaved != 150 {
		// 800→200 tokens raw; 200→50 filtered; saved = 150
		t.Errorf("after reload TokensSaved = %d, want 150", lt.TokensSaved)
	}
}

func TestSessionReuse(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	l1, _ := Load("sess-A")
	_ = l1.RecordCommand("git_status", 400, 100, false)

	l2, _ := Load("sess-A")
	_ = l2.RecordCommand("git_status", 400, 100, false)

	sess := l2.CurrentSession()
	if sess.Commands != 2 {
		t.Errorf("sess.Commands = %d, want 2", sess.Commands)
	}
}

func TestSessionCap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	for i := 0; i < maxSessions+10; i++ {
		l, err := Load(fmt.Sprintf("sess-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		_ = l.Save()
	}

	// Reload and check cap.
	l, _ := Load("final-sess")
	// sessions list should be capped at maxSessions (oldest dropped)
	// final-sess is the last one; there's also up to maxSessions-1 prior
	path := filepath.Join(dir, "wafi", "ledger.json")
	data, _ := os.ReadFile(path)
	var dl diskLedger
	_ = json.Unmarshal(data, &dl)
	if len(dl.Sessions) > maxSessions {
		t.Errorf("sessions len = %d, want <= %d", len(dl.Sessions), maxSessions)
	}
	_ = l
}

func TestCorruptLedgerResets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	ledgerPath := filepath.Join(dir, "wafi", "ledger.json")
	_ = os.MkdirAll(filepath.Dir(ledgerPath), 0o700)
	_ = os.WriteFile(ledgerPath, []byte("not json {{{{"), 0o600)

	l, err := Load("sess-corrupt")
	if err != nil {
		t.Fatalf("Load should succeed after corrupt reset: %v", err)
	}
	lt := l.Lifetime()
	if lt.CommandsFiltered != 0 {
		t.Errorf("expected fresh ledger after corrupt reset")
	}
}

func TestFilterStats(t *testing.T) {
	l, _ := setupTempLedger(t, "sess-stats")

	_ = l.RecordCommand("git_status", 400, 100, false)
	_ = l.RecordCommand("git_status", 800, 200, false)
	_ = l.RecordCommand("npm_install", 1200, 400, false)

	stats := l.FilterStats()
	gs, ok := stats["git_status"]
	if !ok {
		t.Fatal("expected git_status in FilterStats")
	}
	if gs.CallCount != 2 {
		t.Errorf("git_status CallCount = %d, want 2", gs.CallCount)
	}
	// avg raw bytes = (400+800)/2 = 600; /4 = 150 tokens
	if gs.RawAvg != 150 {
		t.Errorf("git_status RawAvg = %v, want 150", gs.RawAvg)
	}
	// avg filtered bytes = (100+200)/2 = 150; /4 = 37.5 tokens
	if gs.FilteredAvg != 37.5 {
		t.Errorf("git_status FilteredAvg = %v, want 37.5", gs.FilteredAvg)
	}
	if gs.SavedAvg != 112.5 {
		t.Errorf("git_status SavedAvg = %v, want 112.5", gs.SavedAvg)
	}

	if _, ok := stats["npm_install"]; !ok {
		t.Fatal("expected npm_install in FilterStats")
	}
}

func TestAtomicWrite(t *testing.T) {
	l, path := setupTempLedger(t, "sess-atomic")
	_ = l.RecordCommand("git_status", 100, 50, false)

	// Tmp file should be gone after save.
	tmp := path + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("expected .tmp file to be cleaned up after atomic rename")
	}
}

func TestDirPermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	_, err := Load("sess-perm")
	if err != nil {
		t.Fatal(err)
	}
	ledgerDir := filepath.Join(dir, "wafi")
	fi, err := os.Stat(ledgerDir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 0700", perm)
	}
}
