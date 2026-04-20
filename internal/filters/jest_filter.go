package filters

import "strings"

// Jest compresses `jest` / `npx jest` output by dropping passing-suite lines.
// " PASS  file.test.ts" lines are redundant — the final summary covers them.
// " FAIL  file.test.ts" lines and their full error details are kept verbatim.
type Jest struct{}

func (Jest) Name() string { return "jest" }

func (Jest) Match(cmd string, args []string) bool {
	if cmd == "jest" {
		return true
	}
	return cmd == "npx" && len(args) > 0 && args[0] == "jest"
}

func (Jest) Apply(out []byte, _ ApplyContext) []byte {
	compact, ok := compressJest(out)
	if !ok {
		return out
	}
	return compact
}

func compressJest(out []byte) ([]byte, bool) {
	text := string(out)
	lines := strings.Split(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	hasPass := false
	for _, line := range lines {
		if strings.HasPrefix(line, " PASS  ") {
			hasPass = true
			break
		}
	}
	if !hasPass {
		return nil, false
	}

	var b strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, " PASS  ") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), true
}
