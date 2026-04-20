package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordRead_NewFile(t *testing.T) {
	s := newTestSession(t)

	content := []byte("hello world")
	seen, rec, err := s.RecordRead("testfile.go", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen {
		t.Fatal("expected alreadySeen=false for first read")
	}
	if rec.ReadCount != 1 {
		t.Fatalf("expected ReadCount=1, got %d", rec.ReadCount)
	}
	if rec.FileHash == "" {
		t.Fatal("expected non-empty FileHash")
	}
	if rec.TokenEst != len(content)/4 {
		t.Fatalf("expected TokenEst=%d, got %d", len(content)/4, rec.TokenEst)
	}
}

func TestRecordRead_SameContentSeenAgain(t *testing.T) {
	s := newTestSession(t)
	content := []byte("unchanged content")

	_, _, err := s.RecordRead("file.go", content)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}

	seen, rec, err := s.RecordRead("file.go", content)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if !seen {
		t.Fatal("expected alreadySeen=true on second read with same content")
	}
	if rec.ReadCount != 2 {
		t.Fatalf("expected ReadCount=2, got %d", rec.ReadCount)
	}
}

func TestRecordRead_ContentChanged(t *testing.T) {
	s := newTestSession(t)

	_, _, err := s.RecordRead("file.go", []byte("original"))
	if err != nil {
		t.Fatalf("first read: %v", err)
	}

	seen, rec, err := s.RecordRead("file.go", []byte("modified"))
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if seen {
		t.Fatal("expected alreadySeen=false when content changed")
	}
	if rec.ReadCount != 2 {
		t.Fatalf("expected ReadCount=2, got %d", rec.ReadCount)
	}
}

func TestRecordRead_RelativePathNormalised(t *testing.T) {
	s := newTestSession(t)
	content := []byte("data")

	_, _, err := s.RecordRead("./some/file.go", content)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}

	// Same file via different relative expression — should be seen.
	seen, _, err := s.RecordRead("some/file.go", content)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if !seen {
		t.Fatal("expected relative paths to resolve to same key")
	}
}

func TestRecordRead_MultipleFiles(t *testing.T) {
	s := newTestSession(t)

	for _, name := range []string{"a.go", "b.go", "c.go"} {
		seen, _, err := s.RecordRead(name, []byte(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if seen {
			t.Fatalf("expected alreadySeen=false for %s", name)
		}
	}

	if len(s.Reads) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(s.Reads))
	}
}

func TestSessionPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sess.json")

	s1 := &Session{
		ID:    "test-sess",
		Reads: make(map[string]ReadRecord),
		path:  path,
	}
	content := []byte("persisted content")
	_, _, err := s1.RecordRead("persist.go", content)
	if err != nil {
		t.Fatalf("record read: %v", err)
	}

	// Reload from disk.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("session file is empty")
	}

	s2 := &Session{Reads: make(map[string]ReadRecord), path: path}
	if err := unmarshalSession(data, s2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(s2.Reads) != 1 {
		t.Fatalf("expected 1 read in reloaded session, got %d", len(s2.Reads))
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("WAFI_SESSION_ID", "my-custom-session")

	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.ID != "my-custom-session" {
		t.Fatalf("expected ID=my-custom-session, got %s", s.ID)
	}
}

func TestSessionID_Sanitize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc-123_XYZ", "abc-123_XYZ"},
		{"foo/bar", "foo_bar"},
		{"", "unknown"},
		{"hello world", "hello_world"},
	}
	for _, c := range cases {
		got := sanitize(c.in)
		if got != c.want {
			t.Errorf("sanitize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// newTestSession creates an in-memory session backed by a temp dir.
func newTestSession(t *testing.T) *Session {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	s := &Session{
		ID:    "test",
		Reads: make(map[string]ReadRecord),
		path:  path,
	}
	return s
}
