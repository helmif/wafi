package filters

import (
	"fmt"
	"strings"
	"testing"
)

func TestDiffMatch(t *testing.T) {
	f := Diff{}
	cases := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"diff", []string{"-u", "file1", "file2"}, true},
		{"diff", []string{"-c", "file1", "file2"}, true},
		{"diff", []string{"file1", "file2"}, true},
		{"diff", []string{}, true},
		// git diff must NOT match — it has its own filter
		{"git", []string{"diff"}, false},
		{"ls", []string{}, false},
	}
	for _, c := range cases {
		got := f.Match(c.cmd, c.args)
		if got != c.want {
			t.Errorf("Match(%q, %v) = %v, want %v", c.cmd, c.args, got, c.want)
		}
	}
}

// fewCtx builds a unified diff string with n context lines before and after the change.
func unifiedDiff(ctxBefore, ctxAfter int) string {
	var b strings.Builder
	b.WriteString("--- a/file.go\t2026-01-01\n")
	b.WriteString("+++ b/file.go\t2026-01-01\n")
	b.WriteString("@@ -1,10 +1,10 @@\n")
	for i := 1; i <= ctxBefore; i++ {
		fmt.Fprintf(&b, " before%d\n", i)
	}
	b.WriteString("-old line\n")
	b.WriteString("+new line\n")
	for i := 1; i <= ctxAfter; i++ {
		fmt.Fprintf(&b, " after%d\n", i)
	}
	return b.String()
}

func TestDiffUnifiedNoExcessContext(t *testing.T) {
	f := Diff{}
	ctx := ApplyContext{Cmd: "diff", Args: []string{"-u", "a", "b"}}

	// 3 context lines each side — exactly at limit, should passthrough unchanged.
	input := unifiedDiff(3, 3)
	got := string(f.Apply([]byte(input), ctx))
	if got != input {
		t.Errorf("unified 3-ctx: expected passthrough, got:\n%s", got)
	}
}

func TestDiffUnifiedExcessContext(t *testing.T) {
	f := Diff{}
	ctx := ApplyContext{Cmd: "diff", Args: []string{"-u", "a", "b"}}

	// 5 context lines each side — excess should be trimmed to 3.
	input := unifiedDiff(5, 5)

	want := "--- a/file.go\t2026-01-01\n" +
		"+++ b/file.go\t2026-01-01\n" +
		"@@ -1,10 +1,10 @@\n" +
		" before1\n before2\n before3\n" +
		"-old line\n+new line\n" +
		" after1\n after2\n after3\n"

	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("unified excess context:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDiffUnifiedMultipleHunks(t *testing.T) {
	f := Diff{}
	ctx := ApplyContext{Cmd: "diff", Args: []string{"-u", "a", "b"}}

	// Two hunks, each with excess context; ctxRun resets at @@ header.
	input := "--- a/file.go\n+++ b/file.go\n" +
		"@@ -1,10 +1,10 @@\n" +
		" c1\n c2\n c3\n c4\n c5\n" + // 5 before, only 3 kept
		"-old1\n+new1\n" +
		" a1\n a2\n a3\n a4\n" + // 4 after, only 3 kept
		"@@ -20,8 +20,8 @@\n" +
		" c1\n c2\n c3\n c4\n" + // 4 before in new hunk, only 3 kept
		"-old2\n+new2\n" +
		" a1\n a2\n"

	want := "--- a/file.go\n+++ b/file.go\n" +
		"@@ -1,10 +1,10 @@\n" +
		" c1\n c2\n c3\n" +
		"-old1\n+new1\n" +
		" a1\n a2\n a3\n" +
		"@@ -20,8 +20,8 @@\n" +
		" c1\n c2\n c3\n" +
		"-old2\n+new2\n" +
		" a1\n a2\n"

	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("unified multi-hunk:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDiffContextFormat(t *testing.T) {
	f := Diff{}
	ctx := ApplyContext{Cmd: "diff", Args: []string{"-c", "a", "b"}}

	input := "*** file1\t2026-01-01\n--- file2\t2026-01-01\n" +
		"***************\n" +
		"*** 1,8 ****\n" +
		"  ctx1\n  ctx2\n  ctx3\n  ctx4\n  ctx5\n" + // 5 before, keep 3
		"! changed old\n" +
		"  after1\n  after2\n  after3\n  after4\n" + // 4 after, keep 3
		"--- 1,8 ----\n" +
		"  ctx1\n  ctx2\n  ctx3\n  ctx4\n" + // 4 before, keep 3
		"! changed new\n" +
		"  after1\n  after2\n  after3\n  after4\n"

	want := "*** file1\t2026-01-01\n--- file2\t2026-01-01\n" +
		"***************\n" +
		"*** 1,8 ****\n" +
		"  ctx1\n  ctx2\n  ctx3\n" +
		"! changed old\n" +
		"  after1\n  after2\n  after3\n" +
		"--- 1,8 ----\n" +
		"  ctx1\n  ctx2\n  ctx3\n" +
		"! changed new\n" +
		"  after1\n  after2\n  after3\n"

	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("context format:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDiffNormalFormat(t *testing.T) {
	f := Diff{}
	ctx := ApplyContext{Cmd: "diff", Args: []string{"a", "b"}}

	// Normal format has no @@ or *** markers — passthrough.
	input := "1,2c1,2\n< old1\n< old2\n---\n> new1\n> new2\n"
	got := string(f.Apply([]byte(input), ctx))
	if got != input {
		t.Errorf("normal format: expected passthrough, got:\n%s", got)
	}
}

func TestDiffNoDifferences(t *testing.T) {
	f := Diff{}
	ctx := ApplyContext{Cmd: "diff", Args: []string{"a", "b"}}

	got := string(f.Apply([]byte(""), ctx))
	if got != "" {
		t.Errorf("no differences: expected passthrough empty, got %q", got)
	}
}

func TestDiffBinaryFiles(t *testing.T) {
	f := Diff{}
	ctx := ApplyContext{Cmd: "diff", Args: []string{"img1.png", "img2.png"}}

	input := "Binary files img1.png and img2.png differ\n"
	got := string(f.Apply([]byte(input), ctx))
	if got != input {
		t.Errorf("binary: expected passthrough, got:\n%s", got)
	}
}

func TestDiffUnifiedNoNewline(t *testing.T) {
	f := Diff{}
	ctx := ApplyContext{Cmd: "diff", Args: []string{"-u", "a", "b"}}

	// "\ No newline" annotation must be preserved.
	input := "--- a\n+++ b\n@@ -1,4 +1,4 @@\n c1\n c2\n c3\n c4\n c5\n-old\n\\ No newline at end of file\n+new\n"
	want := "--- a\n+++ b\n@@ -1,4 +1,4 @@\n c1\n c2\n c3\n" +
		"-old\n\\ No newline at end of file\n+new\n"

	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("no-newline annotation:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDiffMultiFileDiff(t *testing.T) {
	f := Diff{}
	ctx := ApplyContext{Cmd: "diff", Args: []string{"-ru", "dir1", "dir2"}}

	input := "diff -u dir1/a.go dir2/a.go\n" +
		"--- dir1/a.go\n+++ dir2/a.go\n" +
		"@@ -1,6 +1,6 @@\n" +
		" c1\n c2\n c3\n c4\n c5\n" + // 5 before — excess
		"-old\n+new\n"

	want := "diff -u dir1/a.go dir2/a.go\n" +
		"--- dir1/a.go\n+++ dir2/a.go\n" +
		"@@ -1,6 +1,6 @@\n" +
		" c1\n c2\n c3\n" +
		"-old\n+new\n"

	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("multi-file diff:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
