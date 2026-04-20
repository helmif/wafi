package filters

import (
	"regexp"
	"strings"
)

// GitStatus compresses the default (long) form of `git status`.
//
// Matching is intentionally narrow: only a bare `git status` with no flags.
// Any flag could change the output shape (`--porcelain`, `-s`, `--branch`,
// `--column`, `-z`, ...) and we do not want to risk misparsing.
//
// Compression is best-effort and strictly reversible in spirit: if any line
// falls outside the small grammar below, Apply returns the input unchanged.
type GitStatus struct{}

func (GitStatus) Name() string { return "git-status" }

func (GitStatus) Match(cmd string, args []string) bool {
	if cmd != "git" {
		return false
	}
	if len(args) != 1 {
		return false
	}
	return args[0] == "status"
}

func (GitStatus) Apply(out []byte, _ ApplyContext) []byte {
	compact, ok := compressGitStatus(out)
	if !ok {
		return out
	}
	return compact
}

// section tracks which block of `git status` we are currently reading entries
// for. The empty string means we are between sections (header area).
type section string

const (
	secNone      section = ""
	secStaged    section = "staged"
	secUnstaged  section = "unstaged"
	secUntracked section = "untracked"
	secUnmerged  section = "unmerged"
)

// upstreamUpToDate matches "Your branch is up to date with 'origin/main'."
var upstreamUpToDate = regexp.MustCompile(`^Your branch is up to date with '([^']+)'\.$`)

// upstreamAhead matches "Your branch is ahead of 'origin/main' by 3 commits."
var upstreamAhead = regexp.MustCompile(`^Your branch is ahead of '([^']+)' by (\d+) commits?\.$`)

// upstreamBehind matches "Your branch is behind 'origin/main' by 3 commits, and can be fast-forwarded."
var upstreamBehind = regexp.MustCompile(`^Your branch is behind '([^']+)' by (\d+) commits?,`)

// upstreamDivergedHead matches the first line of the "diverged" message.
var upstreamDivergedHead = regexp.MustCompile(`^Your branch and '([^']+)' have diverged,$`)

// upstreamDivergedTail matches "and have 3 and 5 different commits each, respectively."
var upstreamDivergedTail = regexp.MustCompile(`^and have (\d+) and (\d+) different commits each, respectively\.$`)

func compressGitStatus(out []byte) ([]byte, bool) {
	text := string(out)
	lines := strings.Split(text, "\n")

	// Drop a single trailing empty line produced by the terminating newline,
	// so we can treat every remaining empty string as a "blank line" mid-output.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	var (
		branch    string
		upstream  string
		staged    []string
		unstaged  []string
		untracked []string
		unmerged  []string
		finalMsg  string
		sec       = secNone
	)

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Blank lines separate sections; never carry information.
		if line == "" {
			sec = secNone
			continue
		}

		// Branch / HEAD state.
		if strings.HasPrefix(line, "On branch ") {
			if branch != "" {
				return nil, false // duplicate header → unfamiliar shape
			}
			branch = strings.TrimPrefix(line, "On branch ")
			continue
		}
		if strings.HasPrefix(line, "HEAD detached ") {
			if branch != "" {
				return nil, false
			}
			branch = line // preserve verbatim; rarer, keep full info
			continue
		}

		// Upstream status. Handle the two-line "diverged" form by peeking ahead.
		if m := upstreamUpToDate.FindStringSubmatch(line); m != nil {
			upstream = "upstream: " + m[1] + " (clean)"
			continue
		}
		if m := upstreamAhead.FindStringSubmatch(line); m != nil {
			upstream = "upstream: " + m[1] + " (ahead " + m[2] + ")"
			continue
		}
		if m := upstreamBehind.FindStringSubmatch(line); m != nil {
			upstream = "upstream: " + m[1] + " (behind " + m[2] + ")"
			continue
		}
		if m := upstreamDivergedHead.FindStringSubmatch(line); m != nil {
			if i+1 >= len(lines) {
				return nil, false
			}
			t := upstreamDivergedTail.FindStringSubmatch(lines[i+1])
			if t == nil {
				return nil, false
			}
			upstream = "upstream: " + m[1] + " (ahead " + t[1] + ", behind " + t[2] + ")"
			i++
			continue
		}
		if strings.HasPrefix(line, "Your branch ") {
			// Recognized prefix but not a known shape. Bail out rather than
			// risk dropping something the user relies on.
			return nil, false
		}

		// Section headers.
		switch line {
		case "Changes to be committed:":
			sec = secStaged
			continue
		case "Changes not staged for commit:":
			sec = secUnstaged
			continue
		case "Untracked files:":
			sec = secUntracked
			continue
		case "Unmerged paths:":
			sec = secUnmerged
			continue
		}

		// Hint lines inside a section: "  (use \"git ...\")".
		if strings.HasPrefix(line, "  (") && strings.HasSuffix(line, ")") {
			continue
		}

		// Entry lines inside a section are tab-indented.
		if strings.HasPrefix(line, "\t") {
			entry := strings.TrimPrefix(line, "\t")
			switch sec {
			case secStaged:
				e, ok := parseChangeEntry(entry)
				if !ok {
					return nil, false
				}
				staged = append(staged, e)
			case secUnstaged:
				e, ok := parseChangeEntry(entry)
				if !ok {
					return nil, false
				}
				unstaged = append(unstaged, e)
			case secUntracked:
				untracked = append(untracked, entry)
			case secUnmerged:
				e, ok := parseUnmergedEntry(entry)
				if !ok {
					return nil, false
				}
				unmerged = append(unmerged, e)
			default:
				return nil, false
			}
			continue
		}

		// Terminal summary lines.
		if strings.HasPrefix(line, "nothing to commit") ||
			strings.HasPrefix(line, "no changes added to commit") ||
			strings.HasPrefix(line, "nothing added to commit but untracked") {
			finalMsg = line
			continue
		}

		// Anything else is outside the grammar we trust.
		return nil, false
	}

	return renderGitStatus(branch, upstream, staged, unstaged, unmerged, untracked, finalMsg), true
}

// parseChangeEntry reads a staged/unstaged entry like "modified:   foo.go"
// and returns a single compact token: "M foo.go". Returns false for any
// unrecognized label.
func parseChangeEntry(entry string) (string, bool) {
	idx := strings.Index(entry, ":")
	if idx <= 0 {
		return "", false
	}
	label := entry[:idx]
	path := strings.TrimLeft(entry[idx+1:], " \t")
	if path == "" {
		return "", false
	}
	var code string
	switch label {
	case "new file":
		code = "A"
	case "modified":
		code = "M"
	case "deleted":
		code = "D"
	case "renamed":
		code = "R"
	case "copied":
		code = "C"
	case "typechange":
		code = "T"
	default:
		return "", false
	}
	return code + " " + path, true
}

func parseUnmergedEntry(entry string) (string, bool) {
	idx := strings.Index(entry, ":")
	if idx <= 0 {
		return "", false
	}
	label := entry[:idx]
	path := strings.TrimLeft(entry[idx+1:], " \t")
	if path == "" {
		return "", false
	}
	var code string
	switch label {
	case "both modified":
		code = "UU"
	case "both added":
		code = "AA"
	case "both deleted":
		code = "DD"
	case "added by us":
		code = "AU"
	case "deleted by us":
		code = "DU"
	case "added by them":
		code = "UA"
	case "deleted by them":
		code = "UD"
	default:
		return "", false
	}
	return code + " " + path, true
}

func renderGitStatus(branch, upstream string, staged, unstaged, unmerged, untracked []string, finalMsg string) []byte {
	var b strings.Builder
	if branch != "" {
		b.WriteString("branch: ")
		b.WriteString(branch)
		b.WriteByte('\n')
	}
	if upstream != "" {
		b.WriteString(upstream)
		b.WriteByte('\n')
	}
	writeSection(&b, "staged", staged)
	writeSection(&b, "unstaged", unstaged)
	writeSection(&b, "unmerged", unmerged)
	writeSection(&b, "untracked", untracked)
	if len(staged)+len(unstaged)+len(unmerged)+len(untracked) == 0 && finalMsg != "" {
		b.WriteString(finalMsg)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func writeSection(b *strings.Builder, name string, entries []string) {
	if len(entries) == 0 {
		return
	}
	b.WriteString(name)
	b.WriteString(":\n")
	for _, e := range entries {
		b.WriteString("  ")
		b.WriteString(e)
		b.WriteByte('\n')
	}
}
