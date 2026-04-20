package filters

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Ls compresses `ls -l` output by dropping permissions, link count, owner,
// group, and timestamps. Directories are marked with a trailing slash;
// file sizes are converted to human-readable form. Unknown output shapes
// are returned unchanged.
type Ls struct{}

func (Ls) Name() string { return "ls" }

func (Ls) Match(cmd string, args []string) bool {
	if cmd != "ls" {
		return false
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a, 'l') {
			return true
		}
	}
	return false
}

func (Ls) Apply(out []byte, _ ApplyContext) []byte {
	compact, ok := compressLs(out)
	if !ok {
		return out
	}
	return compact
}

// lsLongRe matches a single long-format ls line:
//
//	permissions  links  owner  group  size  month  day  time-or-year  name
//
// Captures: (perm-char-0), (size), (name-including-symlink-arrow).
// Date fields: \w+ (month), \s+\d+ (day with optional extra space), \s+[\d:]+ (time or year).
var lsLongRe = regexp.MustCompile(
	`^([dl\-][rwxstST\-]{9})\s+\d+\s+\S+\s+\S+\s+(\S+)\s+\w+\s+\d+\s+[\d:]+\s+(.+)$`,
)

// lsDirHeaderRe matches recursive section headers like "./path/to/dir:".
var lsDirHeaderRe = regexp.MustCompile(`^(\.?/.*|[^-].*):\s*$`)

func compressLs(out []byte) ([]byte, bool) {
	text := strings.TrimRight(string(out), "\n")
	lines := strings.Split(text, "\n")

	// Require at least one long-format line; otherwise there is nothing to compress.
	hasLong := false
	for _, line := range lines {
		if lsLongRe.MatchString(line) {
			hasLong = true
			break
		}
	}
	if !hasLong {
		return nil, false
	}

	var b strings.Builder
	fileCount := 0

	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "total ") {
			continue
		}
		// Recursive directory section header.
		if lsDirHeaderRe.MatchString(line) {
			if fileCount > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}

		m := lsLongRe.FindStringSubmatch(line)
		if m == nil {
			b.WriteString(line)
			b.WriteByte('\n')
			fileCount++
			continue
		}

		permFirst, sizeStr, name := m[1][0], m[2], m[3]

		// Drop . and .. entries.
		if name == "." || name == ".." {
			continue
		}

		switch permFirst {
		case 'd':
			fmt.Fprintf(&b, "%s/\n", name)
		default:
			fmt.Fprintf(&b, "%s  %s\n", name, lsHumanSize(sizeStr))
		}
		fileCount++
	}

	result := b.String()
	if strings.TrimSpace(result) == "" {
		return []byte("(empty)\n"), true
	}
	return []byte(result), true
}

// lsHumanSize converts a raw byte count string to a compact human-readable
// form. If the string already ends with a SI suffix (K/M/G/T) it is returned
// as-is (ls -h already formatted it).
func lsHumanSize(s string) string {
	if len(s) == 0 {
		return s
	}
	last := s[len(s)-1]
	if last == 'K' || last == 'M' || last == 'G' || last == 'T' || last == 'B' {
		return s
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return s
	}
	const ki = 1024
	switch {
	case n < ki:
		return fmt.Sprintf("%dB", n)
	case n < ki*ki:
		return fmt.Sprintf("%.1fK", float64(n)/ki)
	case n < ki*ki*ki:
		return fmt.Sprintf("%.1fM", float64(n)/(ki*ki))
	default:
		return fmt.Sprintf("%.1fG", float64(n)/(ki*ki*ki))
	}
}
