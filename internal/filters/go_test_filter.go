package filters

import "strings"

// GoTest compresses `go test` output by dropping passing-test lines.
// --- PASS: lines are redundant — the final summary covers them.
// --- FAIL: lines and their indented error context are preserved verbatim.
//
// Match is intentionally narrow: the -v flag changes output shape
// (adds === RUN lines) and is already compact enough to skip.
type GoTest struct{}

func (GoTest) Name() string { return "go-test" }

func (GoTest) Match(cmd string, args []string) bool {
	if cmd != "go" || len(args) < 1 || args[0] != "test" {
		return false
	}
	for _, a := range args[1:] {
		if a == "-v" {
			return false
		}
	}
	return true
}

func (GoTest) Apply(out []byte, _ ApplyContext) []byte {
	compact, ok := compressGoTest(out)
	if !ok {
		return out
	}
	return compact
}

func compressGoTest(out []byte) ([]byte, bool) {
	text := string(out)
	lines := strings.Split(text, "\n")

	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	// Only compress if there are PASS lines to drop; otherwise the output is
	// already compact (no test files, pure panic, build failure, etc.).
	hasPass := false
	for _, line := range lines {
		if strings.HasPrefix(line, "--- PASS:") {
			hasPass = true
			break
		}
	}
	if !hasPass {
		return nil, false
	}

	var b strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "--- PASS:") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), true
}
