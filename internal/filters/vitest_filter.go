package filters

import "strings"

// Vitest compresses `vitest` / `npx vitest` output by dropping passing-file lines.
// " ✓ file.test.ts (N)" lines are redundant — the summary covers them.
// " ✗ " failing-file lines and full error blocks are kept verbatim.
type Vitest struct{}

func (Vitest) Name() string { return "vitest" }

func (Vitest) Match(cmd string, args []string) bool {
	if cmd == "vitest" {
		return true
	}
	return cmd == "npx" && len(args) > 0 && args[0] == "vitest"
}

func (Vitest) Apply(out []byte, _ ApplyContext) []byte {
	compact, ok := compressVitest(out)
	if !ok {
		return out
	}
	return compact
}

func compressVitest(out []byte) ([]byte, bool) {
	text := string(out)
	lines := strings.Split(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	// ✓ is U+2713. Only compress if there are passing-file lines to drop.
	hasPass := false
	for _, line := range lines {
		if strings.HasPrefix(line, " \u2713 ") {
			hasPass = true
			break
		}
	}
	if !hasPass {
		return nil, false
	}

	var b strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, " \u2713 ") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), true
}
