package filters

import (
	"fmt"
	"strings"
	"time"
)

// logNow returns the current time for relative-date computation. Replaced in
// tests so golden files remain stable regardless of when the tests run.
var logNow = func() time.Time { return time.Now() }

// GitLog compresses `git log` output to one line per commit:
//
//	<short-hash> <subject> (<author>, <relative-date>)
//
// Dropped: full hash, email, GPG signatures, body paragraphs, blank lines,
// merge-parent hashes, and (when --stat is used) file-stat lines.
// Preserved: short hash (7 chars), commit subject, author name, relative date.
//
// Match is narrow: plain `git log`, optionally with `--stat`, `--no-color`,
// revision ranges, or `--`. Flags that change the output shape (--oneline,
// --format, --pretty, --word-diff, …) are not matched → pass-through.
//
// Safety: any unrecognised line in the header block triggers pass-through of
// the raw output ("when in doubt, pass-through").
type GitLog struct{}

func (GitLog) Name() string { return "git-log" }

func (GitLog) Match(cmd string, args []string) bool {
	if cmd != "git" || len(args) == 0 || args[0] != "log" {
		return false
	}
	for _, a := range args[1:] {
		switch a {
		case "--stat", "--no-color", "--color=never", "--decorate", "--no-decorate", "--":
			// known safe: body/header format identical to plain log
		default:
			if strings.HasPrefix(a, "-") {
				return false // unknown flag — output shape may differ
			}
			// revision range or path — format unchanged
		}
	}
	return true
}

func (GitLog) Apply(stdout []byte, _ ApplyContext) []byte {
	if len(stdout) == 0 {
		return stdout
	}
	compressed, ok := compressGitLog(stdout)
	if !ok {
		return stdout
	}
	return compressed
}

// compressGitLog parses the default (long) git log format and emits one line
// per commit. Returns (nil, false) on any unrecognised header line so the
// caller can fall back to raw output.
func compressGitLog(out []byte) ([]byte, bool) {
	text := strings.TrimRight(string(out), "\n")
	if text == "" {
		return []byte{}, true
	}

	type logEntry struct {
		hash    string
		author  string
		when    time.Time
		hasDate bool
		subject string
	}

	var commits []logEntry
	var cur *logEntry
	inBody := false

	for _, line := range strings.Split(text, "\n") {
		// Each commit starts with "commit <hash>" optionally followed by
		// decoration: "commit abc123 (HEAD -> main, tag: v1.0)".
		if strings.HasPrefix(line, "commit ") {
			if cur != nil && cur.subject != "" {
				commits = append(commits, *cur)
			}
			raw := strings.TrimPrefix(line, "commit ")
			// Strip any inline decoration after the hash.
			if i := strings.IndexByte(raw, ' '); i > 0 {
				raw = raw[:i]
			}
			if len(raw) < 7 {
				return nil, false // truncated or malformed hash
			}
			cur = &logEntry{hash: raw[:7]}
			inBody = false
			continue
		}

		if cur == nil {
			return nil, false // content before first commit header
		}

		if !inBody {
			switch {
			case strings.HasPrefix(line, "Author: "):
				rest := strings.TrimPrefix(line, "Author: ")
				// "Name Surname <email@host>" — keep only the name part.
				if i := strings.Index(rest, " <"); i >= 0 {
					cur.author = rest[:i]
				} else {
					cur.author = strings.TrimSpace(rest)
				}
			case strings.HasPrefix(line, "Date:   "):
				dateStr := strings.TrimSpace(strings.TrimPrefix(line, "Date:   "))
				// git default date: "Mon Apr 20 16:56:10 2026 +0700"
				// _2 matches both space-padded (" 7") and two-digit ("17") days.
				if t, err := time.Parse("Mon Jan _2 15:04:05 2006 -0700", dateStr); err == nil {
					cur.when = t
					cur.hasDate = true
				}
			case strings.HasPrefix(line, "Merge: "):
				// Merge-parent hashes — informational noise, drop.
			case line == "":
				inBody = true // blank line separates headers from body
			default:
				return nil, false // unexpected header line
			}
			continue
		}

		// Body section: first 4-space-indented line is the subject; everything
		// after (body paragraphs, blank lines, stat lines) is dropped.
		if cur.subject == "" && strings.HasPrefix(line, "    ") {
			cur.subject = strings.TrimSpace(line)
		}
		// All other body/stat lines are silently dropped.
	}

	if cur != nil && cur.subject != "" {
		commits = append(commits, *cur)
	}

	var b strings.Builder
	for _, c := range commits {
		var rel string
		if c.hasDate {
			rel = gitLogRelDate(c.when)
		} else {
			rel = "unknown date"
		}
		fmt.Fprintf(&b, "%s %s (%s, %s)\n", c.hash, c.subject, c.author, rel)
	}
	return []byte(b.String()), true
}

// gitLogRelDate formats t as a human-readable relative duration from logNow.
func gitLogRelDate(t time.Time) string {
	d := logNow().Sub(t)
	if d < 0 {
		d = -d // future timestamp (clock skew) — report absolute delta
	}
	switch {
	case d < 2*time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 2*time.Hour:
		return "1 hour ago"
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 2*24*time.Hour:
		return "1 day ago"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	case d < 2*7*24*time.Hour:
		return "1 week ago"
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/(24*7)))
	case d < 2*30*24*time.Hour:
		return "1 month ago"
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d months ago", int(d.Hours()/(24*30)))
	case d < 2*365*24*time.Hour:
		return "1 year ago"
	default:
		return fmt.Sprintf("%d years ago", int(d.Hours()/(24*365)))
	}
}
