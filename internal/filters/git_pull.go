package filters

import "strings"

// GitPull compresses `git pull` output by dropping remote: progress lines and
// object-unpacking lines.
//
// Kept: "Already up to date.", "From <remote>", "Updating", "Fast-forward",
// file stat lines, "Merge made by", "CONFLICT", "Auto-merging",
// "Automatic merge failed", error: lines, create/delete mode lines.
// Dropped: "remote: " progress, "Unpacking objects:" lines.
//
// Note: git pull writes mix of stdout and stderr. CLI (Phase 11) will merge
// them before passing to this filter.
type GitPull struct{}

func (GitPull) Name() string { return "git-pull" }

func (GitPull) Match(cmd string, args []string) bool {
	if cmd != "git" || len(args) == 0 || args[0] != "pull" {
		return false
	}
	for _, a := range args[1:] {
		if a == "--rebase" || a == "-r" {
			return false // rebase output shape differs
		}
	}
	return true
}

var pullDropPrefixes = []string{
	"remote: ",
	"remote:",
	"Unpacking objects:",
}

func (GitPull) Apply(stdout []byte, _ ApplyContext) []byte {
	if len(stdout) == 0 {
		return stdout
	}
	lines := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if dropPullLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return stdout
	}
	return []byte(strings.Join(kept, "\n") + "\n")
}

func dropPullLine(line string) bool {
	if line == "" {
		return true
	}
	for _, p := range pullDropPrefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}
