package filters

import (
	"regexp"
	"strings"
)

// Diff compresses Unix diff output (not git diff — that is handled by GitDiff).
// Only unified (-u) and context (-c) formats are compressed: consecutive context
// lines are limited to 3 per run. Normal-format diff has no context lines and is
// passed through unchanged. Binary diff notices and empty output are also passed
// through unchanged.
type Diff struct{}

func (Diff) Name() string { return "diff" }

// Match returns true for the standalone `diff` command only. `git diff` is
// cmd="git" args=["diff",...] so it can never match here.
func (Diff) Match(cmd string, args []string) bool { return cmd == "diff" }

func (Diff) Apply(out []byte, _ ApplyContext) []byte {
	compact, ok := compressDiff(out)
	if !ok {
		return out
	}
	return compact
}

var reDiffBinary = regexp.MustCompile(`^Binary files .+ and .+ differ$`)

const diffMaxCtx = 3

func compressDiff(out []byte) ([]byte, bool) {
	text := strings.TrimRight(string(out), "\n")
	if text == "" {
		return nil, false // no differences
	}

	lines := strings.Split(text, "\n")

	// Detect format from the first recognisable structural line.
	format := ""
	for _, line := range lines {
		if reDiffBinary.MatchString(line) {
			return nil, false // binary — passthrough
		}
		if strings.HasPrefix(line, "@@ ") {
			format = "unified"
			break
		}
		if line == "***************" {
			format = "context"
			break
		}
	}
	if format == "" {
		return nil, false // normal format or unrecognised — passthrough
	}

	return compressDiffHunks(lines, format)
}

// compressDiffHunks trims consecutive context lines beyond diffMaxCtx per run.
// A "run" resets whenever a change line or structural header is encountered.
func compressDiffHunks(lines []string, format string) ([]byte, bool) {
	var b strings.Builder
	ctxRun := 0  // current run of consecutive context lines
	dropped := false

	for _, line := range lines {
		switch {
		// ── Unified format ──────────────────────────────────────────────────

		case format == "unified" && (strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ")):
			ctxRun = 0
			b.WriteString(line)
			b.WriteByte('\n')

		case format == "unified" && strings.HasPrefix(line, "@@ "):
			ctxRun = 0
			b.WriteString(line)
			b.WriteByte('\n')

		case format == "unified" && len(line) > 0 && (line[0] == '+' || line[0] == '-'):
			ctxRun = 0
			b.WriteString(line)
			b.WriteByte('\n')

		case format == "unified" && len(line) > 0 && line[0] == ' ':
			ctxRun++
			if ctxRun <= diffMaxCtx {
				b.WriteString(line)
				b.WriteByte('\n')
			} else {
				dropped = true
			}

		// ── Context format ───────────────────────────────────────────────────

		case format == "context" && (strings.HasPrefix(line, "*** ") || strings.HasPrefix(line, "--- ")):
			ctxRun = 0
			b.WriteString(line)
			b.WriteByte('\n')

		case format == "context" && line == "***************":
			ctxRun = 0
			b.WriteString(line)
			b.WriteByte('\n')

		case format == "context" && len(line) >= 2 && (line[:2] == "! " || line[:2] == "+ " || line[:2] == "- "):
			ctxRun = 0
			b.WriteString(line)
			b.WriteByte('\n')

		case format == "context" && strings.HasPrefix(line, "  "):
			ctxRun++
			if ctxRun <= diffMaxCtx {
				b.WriteString(line)
				b.WriteByte('\n')
			} else {
				dropped = true
			}

		// ── Shared: multi-file diff header, "\ No newline" annotations ──────

		case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "\\"):
			ctxRun = 0
			b.WriteString(line)
			b.WriteByte('\n')

		default:
			// Unrecognised line — keep verbatim (safety / zero-risk).
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	if !dropped {
		return nil, false // nothing trimmed — passthrough preserves exact bytes
	}
	return []byte(b.String()), true
}
