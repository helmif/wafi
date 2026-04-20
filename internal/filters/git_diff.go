package filters

import (
	"bytes"
	"strings"
)

// GitDiff compresses `git diff` output by dropping context lines (the
// unchanged lines git shows for context around a hunk). Hunk headers
// (@@ -X,Y +X,Y @@) and all changed lines (+ and -) are preserved so
// the AI can still locate every change precisely.
//
// Match is conservative: only plain `git diff` with optional --staged /
// --cached flags or revision/path arguments. Any flag that could change
// the output shape (--stat, --name-only, --word-diff, …) causes pass-through.
//
// Safety: if Apply sees any line it does not recognise it returns the raw
// input unchanged ("when in doubt, pass-through").
type GitDiff struct{}

func (GitDiff) Name() string { return "git-diff" }

func (GitDiff) Match(cmd string, args []string) bool {
	if cmd != "git" || len(args) == 0 || args[0] != "diff" {
		return false
	}
	for _, a := range args[1:] {
		switch a {
		case "--staged", "--cached", "--no-color", "--color=never", "--":
			// known safe: output format identical to plain diff
		default:
			if strings.HasPrefix(a, "-") {
				return false // unknown flag — output shape may differ
			}
			// revision, branch name, or file path — format unchanged
		}
	}
	return true
}

func (GitDiff) Apply(stdout []byte, _ ApplyContext) []byte {
	if len(stdout) == 0 {
		return stdout // clean diff: no changes, nothing to compress
	}
	compressed, ok := compressGitDiff(stdout)
	if !ok {
		return stdout
	}
	return compressed
}

// compressGitDiff drops context lines from a unified diff. Any unrecognised
// line causes (nil, false) which forces the caller to return raw output.
func compressGitDiff(out []byte) ([]byte, bool) {
	trailingNewline := len(out) > 0 && out[len(out)-1] == '\n'
	text := strings.TrimRight(string(out), "\n")
	lines := strings.Split(text, "\n")

	var b bytes.Buffer
	for _, line := range lines {
		switch classifyDiffLine(line) {
		case diffKeep:
			b.WriteString(line)
			b.WriteByte('\n')
		case diffDrop:
			// context or metadata noise — omit
		case diffUnknown:
			return nil, false
		}
	}

	result := b.Bytes()
	if !trailingNewline && len(result) > 0 {
		result = result[:len(result)-1]
	}
	return result, true
}

type diffLineKind int

const (
	diffKeep    diffLineKind = iota
	diffDrop
	diffUnknown
)

// classifyDiffLine categorises a single line from unified diff output.
// More-specific prefixes are checked before shorter ones ("--- " before "-").
func classifyDiffLine(line string) diffLineKind {
	if line == "" {
		return diffKeep // blank lines between diff sections are valid
	}
	switch {
	// File-level headers.
	case strings.HasPrefix(line, "diff --git "):
		return diffKeep
	case strings.HasPrefix(line, "--- "), strings.HasPrefix(line, "+++ "):
		return diffKeep
	case strings.HasPrefix(line, "@@ "):
		return diffKeep

	// Change lines. "--- " / "+++ " are already matched above so these
	// only fire for actual addition/deletion content lines.
	case line[0] == '+' || line[0] == '-':
		return diffKeep

	// "\ No newline at end of file"
	case strings.HasPrefix(line, "\\ "):
		return diffKeep

	// File-mode and rename/copy metadata — keep.
	case strings.HasPrefix(line, "new file mode"),
		strings.HasPrefix(line, "deleted file mode"),
		strings.HasPrefix(line, "old mode"),
		strings.HasPrefix(line, "new mode"),
		strings.HasPrefix(line, "rename from"),
		strings.HasPrefix(line, "rename to"),
		strings.HasPrefix(line, "copy from"),
		strings.HasPrefix(line, "copy to"),
		strings.HasPrefix(line, "Binary files"):
		return diffKeep

	// Metadata noise — drop silently.
	case strings.HasPrefix(line, "index "),
		strings.HasPrefix(line, "similarity index"),
		strings.HasPrefix(line, "dissimilarity index"):
		return diffDrop

	// Context line (space-prefixed) — the only content we actively remove.
	case line[0] == ' ':
		return diffDrop

	default:
		return diffUnknown
	}
}
