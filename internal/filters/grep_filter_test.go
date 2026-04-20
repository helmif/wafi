package filters

import (
	"testing"
)

func TestGrepMatch(t *testing.T) {
	f := Grep{}
	cases := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"grep", []string{"-rn", "pattern", "."}, true},
		{"grep", []string{"-n", "foo", "file.go"}, true},
		{"grep", []string{"foo", "file.go"}, true},
		{"rg", []string{"pattern"}, true},
		{"find", []string{}, false},
		{"ls", []string{}, false},
	}
	for _, c := range cases {
		got := f.Match(c.cmd, c.args)
		if got != c.want {
			t.Errorf("Match(%q, %v) = %v, want %v", c.cmd, c.args, got, c.want)
		}
	}
}

func TestGrepMultiFileWithLineNumbers(t *testing.T) {
	f := Grep{}
	ctx := ApplyContext{Cmd: "grep", Args: []string{"-rn", "validateToken", "."}}

	input := `src/auth.go:42:func validateToken(token string) bool {
src/auth.go:89:	token := ctx.Value(tokenKey)
src/auth.go:134:	if !validateToken(tok) {
src/main.go:5:import "auth"
`
	want := `src/auth.go (3 matches)
  42: func validateToken(token string) bool {
  89: 	token := ctx.Value(tokenKey)
  134: 	if !validateToken(tok) {
src/main.go (1 match)
  5: import "auth"
`
	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("multi-file Apply():\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGrepContextLimiting(t *testing.T) {
	f := Grep{}
	ctx := ApplyContext{Cmd: "grep", Args: []string{"-rn", "-C", "5", "match", "."}}

	// 5 context lines before and after — only 2 should survive.
	input := `src/foo.go:10:ctx1
src/foo.go:11:ctx2
src/foo.go:12:ctx3
src/foo.go:13:ctx4
src/foo.go:14:ctx5
src/foo.go:15:MATCH LINE
src/foo.go:16:after1
src/foo.go:17:after2
src/foo.go:18:after3
src/foo.go:19:after4
src/foo.go:20:after5
`
	// Wait: context lines use '-' separator, match lines use ':' — let me fix the input
	_ = input
	input = "src/foo.go-10-ctx1\n" +
		"src/foo.go-11-ctx2\n" +
		"src/foo.go-12-ctx3\n" +
		"src/foo.go-13-ctx4\n" +
		"src/foo.go-14-ctx5\n" +
		"src/foo.go:15:MATCH LINE\n" +
		"src/foo.go-16-after1\n" +
		"src/foo.go-17-after2\n" +
		"src/foo.go-18-after3\n" +
		"src/foo.go-19-after4\n" +
		"src/foo.go-20-after5\n"

	want := `src/foo.go (1 match)
  13: ctx4
  14: ctx5
  15: MATCH LINE
  16: after1
  17: after2
`
	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("context-limiting Apply():\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGrepBinaryMatches(t *testing.T) {
	f := Grep{}
	ctx := ApplyContext{Cmd: "grep", Args: []string{"-r", "foo", "."}}

	input := `Binary file ./build/app matches
Binary file ./assets/logo.png matches
src/main.go:5:foo bar
`
	want := `[binary: ./build/app]
[binary: ./assets/logo.png]
src/main.go (1 match)
  5: foo bar
`
	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("binary Apply():\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGrepBinaryOnly(t *testing.T) {
	f := Grep{}
	ctx := ApplyContext{Cmd: "grep", Args: []string{"-r", "foo", "."}}

	input := "Binary file ./data.bin matches\nBinary file ./other.bin matches\n"
	want := "[binary: ./data.bin]\n[binary: ./other.bin]\n"
	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("binary-only Apply():\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGrepNoMatches(t *testing.T) {
	f := Grep{}
	ctx := ApplyContext{Cmd: "grep", Args: []string{"-rn", "missing", "."}}

	input := ""
	got := string(f.Apply([]byte(input), ctx))
	if got != "" {
		t.Errorf("empty input: expected passthrough, got %q", got)
	}
}

func TestGrepSingleFileWithLineNumbers(t *testing.T) {
	f := Grep{}
	ctx := ApplyContext{Cmd: "grep", Args: []string{"-n", "func", "main.go"}}

	// Few matches, no context — should passthrough unchanged.
	input := "5:func main() {\n42:func helper() {\n"
	got := string(f.Apply([]byte(input), ctx))
	if got != input {
		t.Errorf("single-file no context: expected passthrough, got:\n%s", got)
	}
}

func TestGrepSingleFileExcessContext(t *testing.T) {
	f := Grep{}
	ctx := ApplyContext{Cmd: "grep", Args: []string{"-n", "-C", "5", "func", "main.go"}}

	input := "1-line1\n2-line2\n3-line3\n4-line4\n5-line5\n6:func main() {\n7-line7\n8-line8\n9-line9\n10-line10\n11-line11\n"
	want := "4: line4\n5: line5\n6: func main() {\n7: line7\n8: line8\n"
	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("single-file excess context Apply():\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGrepUnknownFormat(t *testing.T) {
	f := Grep{}
	ctx := ApplyContext{Cmd: "grep", Args: []string{"foo"}}

	// Plain content without filename or line number prefix — passthrough.
	input := "just some matched content\nanother line\n"
	got := string(f.Apply([]byte(input), ctx))
	if got != input {
		t.Errorf("unknown format: expected passthrough, got:\n%s", got)
	}
}

func TestGrepWithSeparator(t *testing.T) {
	f := Grep{}
	ctx := ApplyContext{Cmd: "grep", Args: []string{"-rn", "-C", "1", "match", "."}}

	// grep --group-separator=-- between disjoint match groups in same file.
	input := "src/a.go:10:first match\nsrc/a.go-11-after1\n--\nsrc/a.go-50-before1\nsrc/a.go:51:second match\n"
	want := `src/a.go (2 matches)
  10: first match
  11: after1
  50: before1
  51: second match
`
	got := string(f.Apply([]byte(input), ctx))
	if got != want {
		t.Errorf("separator Apply():\ngot:\n%s\nwant:\n%s", got, want)
	}
}
