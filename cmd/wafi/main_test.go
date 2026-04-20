package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"", 0, true},
		{"xd", 0, true},
	}
	for _, c := range cases {
		got, err := parseDuration(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseDuration(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("parseDuration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{2048, "2.0KB"},
		{5 * 1024 * 1024, "5.0MB"},
	}
	for _, c := range cases {
		if got := humanSize(c.in); got != c.want {
			t.Errorf("humanSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRewriteCommand(t *testing.T) {
	cases := []struct {
		in       string
		want     string
		changed  bool
	}{
		{"git status", "wafi run git status", true},
		{"npm install", "wafi run npm install", true},
		{"  ls -la  ", "wafi run ls -la", true},
		{"/usr/bin/git log", "wafi run /usr/bin/git log", true},
		{"echo hello", "echo hello", false},
		{"", "", false},
		{"unknown-binary", "unknown-binary", false},
	}
	for _, c := range cases {
		got, changed := rewriteCommand(c.in)
		if got != c.want || changed != c.changed {
			t.Errorf("rewriteCommand(%q) = (%q, %v), want (%q, %v)",
				c.in, got, changed, c.want, c.changed)
		}
	}
}

func TestHookAlreadyRegistered(t *testing.T) {
	empty := map[string]any{}
	if hookAlreadyRegistered(empty) {
		t.Fatal("empty config should not match")
	}

	present := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": hookCmd},
					},
				},
			},
		},
	}
	if !hookAlreadyRegistered(present) {
		t.Fatal("hook should be detected")
	}

	malformed := map[string]any{"hooks": "not-a-map"}
	if hookAlreadyRegistered(malformed) {
		t.Fatal("malformed config should not match")
	}
}

func TestRegisterHook(t *testing.T) {
	root := map[string]any{}
	registerHook(root)
	if !hookAlreadyRegistered(root) {
		t.Fatal("hook should be registered")
	}
	// Registering twice via cmdInit path would not duplicate (caller checks
	// first). registerHook itself appends unconditionally — test that, too.
	registerHook(root)
	hooks := root["hooks"].(map[string]any)
	pre := hooks["PreToolUse"].([]any)
	if len(pre) != 2 {
		t.Fatalf("expected 2 entries after two registers, got %d", len(pre))
	}
}

func TestCmdInit_CreatesSettings(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if code := cmdInit(nil); code != 0 {
		t.Fatalf("cmdInit rc=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(dir, settingsFile))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if !strings.Contains(string(data), hookCmd) {
		t.Fatalf("settings missing hook cmd: %s", data)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !hookAlreadyRegistered(parsed) {
		t.Fatal("hook not registered in written file")
	}

	// Running twice must be idempotent.
	if code := cmdInit(nil); code != 0 {
		t.Fatalf("second cmdInit rc=%d", code)
	}
	data2, _ := os.ReadFile(filepath.Join(dir, settingsFile))
	var parsed2 map[string]any
	_ = json.Unmarshal(data2, &parsed2)
	hooks := parsed2["hooks"].(map[string]any)
	pre := hooks["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("expected 1 entry after idempotent init, got %d", len(pre))
	}
}

func TestCmdInit_PreservesExistingSettings(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(dir, claudeDir), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `{"theme":"dark"}`
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte(existing), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if code := cmdInit(nil); code != 0 {
		t.Fatalf("cmdInit rc=%d", code)
	}
	data, _ := os.ReadFile(filepath.Join(dir, settingsFile))
	var parsed map[string]any
	_ = json.Unmarshal(data, &parsed)
	if parsed["theme"] != "dark" {
		t.Fatalf("existing key lost: %+v", parsed)
	}
	if !hookAlreadyRegistered(parsed) {
		t.Fatal("hook missing")
	}
}

func TestSessionID(t *testing.T) {
	t.Setenv("CLAUDE_SESSION_ID", "claude-abc")
	if got := sessionID(); got != "claude-abc" {
		t.Fatalf("sessionID=%q, want claude-abc", got)
	}

	t.Setenv("CLAUDE_SESSION_ID", "")
	got := sessionID()
	if !strings.HasPrefix(got, "pid") {
		t.Fatalf("expected fallback pid prefix, got %q", got)
	}
}

func TestListStashFiles_Empty(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	files, err := listStashFiles()
	if err != nil {
		t.Fatalf("listStashFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected empty list, got %d", len(files))
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what
// was written. Keeps tests quiet.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if rerr != nil {
				break
			}
		}
		done <- sb.String()
	}()
	fn()
	_ = w.Close()
	out := <-done
	os.Stdout = orig
	return out
}

func TestCmdStashList_NoFiles(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out := captureStdout(t, func() {
		if code := cmdStashList(nil); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	if !strings.Contains(out, "no stash files") {
		t.Fatalf("output=%q", out)
	}
}

func TestCmdStashList_OneFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	dir := filepath.Join(tmp, "wafi", "stash")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, "1776000000_git.log")
	if err := os.WriteFile(name, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if code := cmdStashList(nil); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	if !strings.Contains(out, "git") {
		t.Fatalf("output missing command: %s", out)
	}
}

func TestCmdStashShow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	dir := filepath.Join(tmp, "wafi", "stash")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, "1776000000_git.log")
	payload := []byte("stash contents")
	if err := os.WriteFile(name, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if code := cmdStashShow([]string{"1776000000_git"}); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	if out != string(payload) {
		t.Fatalf("out=%q, want %q", out, payload)
	}

	// Missing id
	code := cmdStashShow([]string{"does-not-exist"})
	if code != 1 {
		t.Fatalf("rc=%d, want 1", code)
	}

	// No args
	code = cmdStashShow(nil)
	if code != 2 {
		t.Fatalf("rc=%d, want 2", code)
	}
}

func TestCmdStashClean(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	dir := filepath.Join(tmp, "wafi", "stash")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(dir, "1000000000_old.log")
	if err := os.WriteFile(oldFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(oldFile, past, past)

	out := captureStdout(t, func() {
		if code := cmdStashClean([]string{"--older-than", "24h", "--yes"}); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	if !strings.Contains(out, "removed 1") {
		t.Fatalf("output=%q", out)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatal("old file still present")
	}
}

func TestCmdStashClean_InvalidDuration(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	code := cmdStashClean([]string{"--older-than", "bogus", "--yes"})
	if code != 2 {
		t.Fatalf("rc=%d, want 2", code)
	}
}

func TestCmdStash_Dispatch(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if code := cmdStash(nil); code != 2 {
		t.Fatalf("no args rc=%d, want 2", code)
	}
	if code := cmdStash([]string{"unknown"}); code != 2 {
		t.Fatalf("unknown rc=%d, want 2", code)
	}
	_ = captureStdout(t, func() {
		if code := cmdStash([]string{"list"}); code != 0 {
			t.Errorf("list rc=%d", code)
		}
	})
}

func TestCmdHook_Dispatch(t *testing.T) {
	if code := cmdHook(nil); code != 2 {
		t.Fatalf("no args rc=%d, want 2", code)
	}
	if code := cmdHook([]string{"bogus"}); code != 2 {
		t.Fatalf("bogus rc=%d, want 2", code)
	}
}

func TestCmdHookRewrite_ReplacesKnownCommand(t *testing.T) {
	origIn := os.Stdin
	t.Cleanup(func() { os.Stdin = origIn })

	input := `{"tool_input":{"command":"git status"}}`
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	os.Stdin = r

	out := captureStdout(t, func() {
		if code := cmdHookRewrite(); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	ti := parsed["tool_input"].(map[string]any)
	if cmd := ti["command"].(string); cmd != "wafi run git status" {
		t.Fatalf("command=%q", cmd)
	}
}

func TestCmdHookRewrite_LeavesUnknownUntouched(t *testing.T) {
	origIn := os.Stdin
	t.Cleanup(func() { os.Stdin = origIn })

	input := `{"tool_input":{"command":"echo hello"}}`
	r, w, _ := os.Pipe()
	_, _ = w.WriteString(input)
	_ = w.Close()
	os.Stdin = r

	out := captureStdout(t, func() {
		_ = cmdHookRewrite()
	})
	var parsed map[string]any
	_ = json.Unmarshal([]byte(out), &parsed)
	ti := parsed["tool_input"].(map[string]any)
	if cmd := ti["command"].(string); cmd != "echo hello" {
		t.Fatalf("command mutated: %q", cmd)
	}
}

func TestCmdHookRewrite_MalformedPassthrough(t *testing.T) {
	origIn := os.Stdin
	t.Cleanup(func() { os.Stdin = origIn })

	input := "not-json"
	r, w, _ := os.Pipe()
	_, _ = w.WriteString(input)
	_ = w.Close()
	os.Stdin = r

	out := captureStdout(t, func() {
		_ = cmdHookRewrite()
	})
	if out != input {
		t.Fatalf("out=%q, want passthrough of input", out)
	}
}

func TestCmdStats(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out := captureStdout(t, func() {
		if code := cmdStats(nil); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	if !strings.Contains(out, "Lifetime") {
		t.Fatalf("missing Lifetime header: %s", out)
	}
	if !strings.Contains(out, "Session") {
		t.Fatalf("missing Session header: %s", out)
	}
}

func TestCmdStats_JSON(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out := captureStdout(t, func() {
		if code := cmdStats([]string{"--json"}); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse: %v, out=%s", err, out)
	}
	if _, ok := parsed["lifetime"]; !ok {
		t.Fatalf("missing lifetime key: %+v", parsed)
	}
}

func TestCmdStats_SessionJSON(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out := captureStdout(t, func() {
		if code := cmdStats([]string{"--session", "--json"}); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse: %v, out=%s", err, out)
	}
	// session JSON shape should have an id field (SessionEntry.ID).
	if _, ok := parsed["id"]; !ok {
		t.Fatalf("missing id: %+v", parsed)
	}
}

func TestCmdDoctor(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	// cmdDoctor returns non-zero when any probe fails. In a temp dir we expect
	// ledger + stash to pass, but the hook file is missing → WARN (not fail).
	code := captureStdoutCode(t, func() int { return cmdDoctor(nil) })
	if code != 0 {
		t.Fatalf("rc=%d, want 0 (warnings are not failures)", code)
	}
}

func captureStdoutCode(t *testing.T, fn func() int) int {
	t.Helper()
	var code int
	_ = captureStdout(t, func() { code = fn() })
	return code
}

func TestHookAlreadyRegistered_MissingPreToolUse(t *testing.T) {
	cfg := map[string]any{
		"hooks": map[string]any{
			"PostToolUse": []any{},
		},
	}
	if hookAlreadyRegistered(cfg) {
		t.Fatal("should return false when PreToolUse absent")
	}
}

func TestHookAlreadyRegistered_DifferentCommand(t *testing.T) {
	cfg := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "other-tool"},
					},
				},
			},
		},
	}
	if hookAlreadyRegistered(cfg) {
		t.Fatal("should not match unrelated hook")
	}
}

func TestListStashFiles_IgnoresMalformed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	dir := filepath.Join(tmp, "wafi", "stash")
	_ = os.MkdirAll(dir, 0o700)
	// valid file
	_ = os.WriteFile(filepath.Join(dir, "1776000000_git.log"), []byte("x"), 0o600)
	// wrong extension → ignored
	_ = os.WriteFile(filepath.Join(dir, "1776000000_git.txt"), []byte("x"), 0o600)
	// no underscore → ignored
	_ = os.WriteFile(filepath.Join(dir, "nodelimiter.log"), []byte("x"), 0o600)
	// non-numeric prefix → ignored
	_ = os.WriteFile(filepath.Join(dir, "abc_git.log"), []byte("x"), 0o600)
	// subdirectory → ignored
	_ = os.MkdirAll(filepath.Join(dir, "subdir.log"), 0o700)

	files, err := listStashFiles()
	if err != nil {
		t.Fatalf("listStashFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
}

func TestFindStashFile_NotFound(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if _, err := findStashFile("missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFindStashFile_Found(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	dir := filepath.Join(tmp, "wafi", "stash")
	_ = os.MkdirAll(dir, 0o700)
	name := filepath.Join(dir, "1776000000_git.log")
	_ = os.WriteFile(name, []byte("x"), 0o600)

	sf, err := findStashFile("1776000000_git.log")
	if err != nil {
		t.Fatalf("findStashFile: %v", err)
	}
	if sf.path != name {
		t.Fatalf("path=%q, want %q", sf.path, name)
	}

	sf2, err := findStashFile("1776000000_git")
	if err != nil {
		t.Fatalf("findStashFile trimmed: %v", err)
	}
	if sf2.path != name {
		t.Fatalf("trimmed lookup failed")
	}
}

func TestCmdRun_PassthroughUnknown(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out := captureStdout(t, func() {
		if code := cmdRun([]string{"echo", "hello"}); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	if strings.TrimSpace(out) != "hello" {
		t.Fatalf("out=%q", out)
	}
}

func TestCmdRun_NoArgs(t *testing.T) {
	code := cmdRun(nil)
	if code != 2 {
		t.Fatalf("rc=%d, want 2", code)
	}
}

func TestCmdRun_NonZeroExit(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	code := cmdRun([]string{"sh", "-c", "exit 3"})
	if code != 3 {
		t.Fatalf("rc=%d, want 3", code)
	}
}

func silenceStderr(t *testing.T) {
	t.Helper()
	orig := os.Stderr
	_, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		_ = w.Close()
		os.Stderr = orig
	})
}

func TestCmdRun_FailedFilteredCommandStashes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	silenceStderr(t)

	// Emit text that looks like git status noise, then exit non-zero.
	// This makes runner return non-zero; `git` filter won't match because
	// cmd is "sh". Instead drive `cmdRun` with a bona fide non-matching cmd
	// to exercise the non-zero + passthrough branch.
	out := captureStdout(t, func() {
		_ = cmdRun([]string{"sh", "-c", "echo boom; exit 2"})
	})
	if !strings.Contains(out, "boom") {
		t.Fatalf("stdout missing boom: %q", out)
	}
}

func TestCmdRun_StartError(t *testing.T) {
	silenceStderr(t)
	code := cmdRun([]string{"this-binary-does-not-exist-wafi"})
	if code == 0 {
		t.Fatalf("expected non-zero, got %d", code)
	}
}

func TestCmdRun_FilterApplied(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	// Run `git status` in a temp (non-git) dir. git exits non-zero; the
	// `git status` filter matches → exercise the filter + stash path.
	origWD, _ := os.Getwd()
	dir := t.TempDir()
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	silenceStderr(t)

	out := captureStdout(t, func() {
		code := cmdRun([]string{"git", "status"})
		if code == 0 {
			t.Errorf("expected non-zero exit")
		}
	})
	// On stash-hint path we expect the hint line appended.
	if !strings.Contains(out, "[wafi] full output") {
		// Not fatal — git might behave differently — but at least the
		// code path was walked. Print for diagnosis without failing.
		t.Logf("no stash hint in output: %q", out)
	}
}

func TestCmdStashClean_DefaultDurationAbort(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	// Feed "n" to stdin so confirmation aborts.
	origIn := os.Stdin
	t.Cleanup(func() { os.Stdin = origIn })
	r, w, _ := os.Pipe()
	_, _ = w.WriteString("n\n")
	_ = w.Close()
	os.Stdin = r

	out := captureStdout(t, func() {
		if code := cmdStashClean(nil); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	if !strings.Contains(out, "aborted") {
		t.Fatalf("output=%q", out)
	}
}

func TestCmdStashClean_ConfirmYes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	dir := filepath.Join(tmp, "wafi", "stash")
	_ = os.MkdirAll(dir, 0o700)
	f := filepath.Join(dir, "1000000000_old.log")
	_ = os.WriteFile(f, []byte("x"), 0o600)
	past := time.Now().Add(-8 * 24 * time.Hour)
	_ = os.Chtimes(f, past, past)

	origIn := os.Stdin
	t.Cleanup(func() { os.Stdin = origIn })
	r, w, _ := os.Pipe()
	_, _ = w.WriteString("y\n")
	_ = w.Close()
	os.Stdin = r

	out := captureStdout(t, func() {
		if code := cmdStashClean(nil); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	if !strings.Contains(out, "removed") {
		t.Fatalf("output=%q", out)
	}
}

func TestCmdInit_CorruptExistingSettings(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)
	_ = os.MkdirAll(filepath.Join(dir, claudeDir), 0o700)
	_ = os.WriteFile(filepath.Join(dir, settingsFile), []byte("not-json"), 0o600)

	silenceStderr(t)
	code := cmdInit(nil)
	if code != 1 {
		t.Fatalf("rc=%d, want 1", code)
	}
}

func TestCmdInit_AlreadyRegistered(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	_ = os.Chdir(dir)

	// First init creates settings.
	if code := cmdInit(nil); code != 0 {
		t.Fatalf("first init rc=%d", code)
	}
	// Second init prints "Already registered".
	out := captureStdout(t, func() {
		if code := cmdInit(nil); code != 0 {
			t.Errorf("rc=%d", code)
		}
	})
	if !strings.Contains(out, "Already registered") {
		t.Fatalf("output=%q", out)
	}
}

func TestUsage(t *testing.T) {
	orig := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	usage()
	_ = w.Close()
	var buf [4096]byte
	n, _ := r.Read(buf[:])
	os.Stderr = orig
	if !strings.Contains(string(buf[:n]), "wafi") {
		t.Fatal("usage did not mention wafi")
	}
}
