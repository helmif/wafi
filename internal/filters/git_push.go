package filters

import "strings"

// GitPush compresses `git push` output by dropping progress and remote: lines.
//
// Kept: "To <remote>", branch ref update lines (new branch, rejected, updated),
// "Everything up-to-date", error: and hint: lines.
// Dropped: "remote: " progress, object counting/compression/writing lines.
//
// Note: git push writes to stderr, not stdout. In practice the CLI (Phase 11)
// will merge stderr into the stream passed to this filter for push/pull.
type GitPush struct{}

func (GitPush) Name() string { return "git-push" }

func (GitPush) Match(cmd string, args []string) bool {
	if cmd != "git" || len(args) == 0 || args[0] != "push" {
		return false
	}
	for _, a := range args[1:] {
		if a == "--porcelain" {
			return false // machine-readable format, different shape
		}
	}
	return true
}

var pushDropPrefixes = []string{
	"remote: ",
	"remote:",
	"Enumerating objects:",
	"Counting objects:",
	"Delta compression",
	"Compressing objects:",
	"Writing objects:",
	"Total ",
}

func (GitPush) Apply(stdout []byte, _ ApplyContext) []byte {
	if len(stdout) == 0 {
		return stdout
	}
	lines := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if dropPushLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return stdout
	}
	return []byte(strings.Join(kept, "\n") + "\n")
}

func dropPushLine(line string) bool {
	if line == "" {
		return true
	}
	for _, p := range pushDropPrefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}
