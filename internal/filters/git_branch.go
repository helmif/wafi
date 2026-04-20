package filters

import "strings"

// GitBranch reformats `git branch` and `git branch -a` output.
//
// Local branches are kept as-is (with * for current branch).
// Remote tracking branches are reformatted: "  remotes/origin/feat" →
// "  [origin] feat", making the remote name obvious without the prefix noise.
// HEAD pointer lines ("  remotes/origin/HEAD -> ...") are dropped.
//
// Mutation flags (-d, --delete, -m, --move, etc.) and verbose flags (-v, -vv)
// are not matched — they change the output shape or perform writes.
type GitBranch struct{}

func (GitBranch) Name() string { return "git-branch" }

var branchSkipFlags = map[string]bool{
	// mutations
	"-d": true, "--delete": true,
	"-D": true,
	"-m": true, "--move": true,
	"-M": true,
	"-c": true, "--copy": true,
	"-C": true,
	"-u": true, "--set-upstream-to": true,
	"--unset-upstream":    true,
	"--edit-description": true,
	// verbose changes output shape
	"-v": true, "--verbose": true,
	"-vv": true,
}

func (GitBranch) Match(cmd string, args []string) bool {
	if cmd != "git" || len(args) == 0 || args[0] != "branch" {
		return false
	}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") && branchSkipFlags[a] {
			return false
		}
	}
	return true
}

func (GitBranch) Apply(stdout []byte, _ ApplyContext) []byte {
	if len(stdout) == 0 {
		return stdout
	}
	lines := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Drop HEAD pointer lines: "  remotes/origin/HEAD -> origin/main"
		if strings.Contains(line, "HEAD ->") {
			continue
		}
		// Reformat remote tracking branches.
		// They look like "  remotes/origin/branch-name" (two leading spaces).
		if after, ok := strings.CutPrefix(line, "  remotes/"); ok {
			if i := strings.IndexByte(after, '/'); i >= 0 {
				remote := after[:i]
				branch := after[i+1:]
				kept = append(kept, "  ["+remote+"] "+branch)
				continue
			}
			// No slash → unusual format, keep as-is
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return stdout
	}
	return []byte(strings.Join(kept, "\n") + "\n")
}
