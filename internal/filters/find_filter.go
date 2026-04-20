package filters

import (
	"fmt"
	"strings"
)

// Find truncates large `find` output and strips permission-denied noise.
// Short results (≤50 lines, no denied errors) are returned unchanged so the
// filter adds zero overhead when output is already compact.
type Find struct{}

func (Find) Name() string { return "find" }

func (Find) Match(cmd string, args []string) bool { return cmd == "find" }

func (Find) Apply(out []byte, _ ApplyContext) []byte {
	compact, ok := compressFind(out)
	if !ok {
		return out
	}
	return compact
}

const (
	findPassThru  = 50 // line count at or below which output is kept verbatim
	findHeadLines = 40 // lines shown when truncation kicks in
)

func compressFind(out []byte) ([]byte, bool) {
	text := strings.TrimRight(string(out), "\n")
	if text == "" {
		return nil, false
	}

	lines := strings.Split(text, "\n")

	var kept []string
	denied := 0
	for _, line := range lines {
		if strings.Contains(line, ": Permission denied") ||
			strings.Contains(line, ": Operation not permitted") {
			denied++
			continue
		}
		kept = append(kept, line)
	}

	needsTruncation := len(kept) > findPassThru
	if !needsTruncation && denied == 0 {
		return nil, false // already compact, no noise to strip
	}

	var b strings.Builder
	shown := kept
	extra := 0
	if needsTruncation {
		shown = kept[:findHeadLines]
		extra = len(kept) - findHeadLines
	}
	for _, line := range shown {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if extra > 0 {
		fmt.Fprintf(&b, "... (+%d more)\n", extra)
	}
	if denied > 0 {
		fmt.Fprintf(&b, "[%d permission denied]\n", denied)
	}
	return []byte(b.String()), true
}
