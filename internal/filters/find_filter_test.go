package filters

import (
	"fmt"
	"strings"
	"testing"
)

func TestFindMatch(t *testing.T) {
	f := Find{}
	cases := []struct {
		cmd  string
		args []string
		want bool
	}{
		{"find", []string{".", "-name", "*.go"}, true},
		{"find", []string{".", "-type", "f"}, true},
		{"find", []string{"/usr"}, true},
		{"find", []string{}, true},
		{"ls", []string{}, false},
		{"grep", []string{}, false},
	}
	for _, c := range cases {
		got := f.Match(c.cmd, c.args)
		if got != c.want {
			t.Errorf("Match(%q, %v) = %v, want %v", c.cmd, c.args, got, c.want)
		}
	}
}

func TestFindApply(t *testing.T) {
	f := Find{}
	ctx := ApplyContext{Cmd: "find", Args: []string{".", "-name", "*.go"}}

	t.Run("short output passthrough", func(t *testing.T) {
		var lines []string
		for i := 0; i < 30; i++ {
			lines = append(lines, fmt.Sprintf("./pkg/file%d.go", i))
		}
		input := strings.Join(lines, "\n") + "\n"
		got := string(f.Apply([]byte(input), ctx))
		if got != input {
			t.Errorf("expected passthrough, got different output")
		}
	})

	t.Run("exactly 50 lines passthrough", func(t *testing.T) {
		var lines []string
		for i := 0; i < 50; i++ {
			lines = append(lines, fmt.Sprintf("./file%d.go", i))
		}
		input := strings.Join(lines, "\n") + "\n"
		got := string(f.Apply([]byte(input), ctx))
		if got != input {
			t.Errorf("expected passthrough at threshold, got different output")
		}
	})

	t.Run("large output truncated", func(t *testing.T) {
		var lines []string
		for i := 0; i < 80; i++ {
			lines = append(lines, fmt.Sprintf("./file%d.go", i))
		}
		input := strings.Join(lines, "\n") + "\n"
		got := string(f.Apply([]byte(input), ctx))

		gotLines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		// 40 file lines + 1 summary line
		if len(gotLines) != 41 {
			t.Errorf("expected 41 lines (40 + summary), got %d", len(gotLines))
		}
		if !strings.HasPrefix(gotLines[40], "... (+40 more)") {
			t.Errorf("expected truncation summary, got: %q", gotLines[40])
		}
	})

	t.Run("permission denied stripped", func(t *testing.T) {
		input := `./readable/file.go
find: ./secret: Permission denied
./other/file.go
find: /root: Operation not permitted
`
		want := `./readable/file.go
./other/file.go
[2 permission denied]
`
		got := string(f.Apply([]byte(input), ctx))
		if got != want {
			t.Errorf("Apply():\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("large output with denied", func(t *testing.T) {
		var lines []string
		for i := 0; i < 60; i++ {
			lines = append(lines, fmt.Sprintf("./file%d.go", i))
		}
		lines = append(lines, "find: ./nope: Permission denied")
		input := strings.Join(lines, "\n") + "\n"
		got := string(f.Apply([]byte(input), ctx))

		if !strings.Contains(got, "... (+20 more)") {
			t.Errorf("expected truncation summary in: %s", got)
		}
		if !strings.Contains(got, "[1 permission denied]") {
			t.Errorf("expected denied note in: %s", got)
		}
	})

	t.Run("empty output passthrough", func(t *testing.T) {
		got := string(f.Apply([]byte(""), ctx))
		if got != "" {
			t.Errorf("expected empty passthrough, got: %q", got)
		}
	})
}
