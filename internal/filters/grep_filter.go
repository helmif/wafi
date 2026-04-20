package filters

import (
	"fmt"
	"regexp"
	"strings"
)

// Grep compresses grep / rg output by grouping matches by file and trimming
// context lines to at most 2 before/after each match. Binary-file notices are
// collapsed to a one-line note. Unknown output shapes are returned unchanged.
type Grep struct{}

func (Grep) Name() string { return "grep" }

func (Grep) Match(cmd string, args []string) bool {
	return cmd == "grep" || cmd == "rg"
}

func (Grep) Apply(out []byte, _ ApplyContext) []byte {
	compact, ok := compressGrep(out)
	if !ok {
		return out
	}
	return compact
}

var (
	reGrepBinary = regexp.MustCompile(`^Binary file (.+) matches$`)

	// Multi-file with line numbers. Filename must not start with a digit so
	// we don't collide with single-file "N:content" lines.
	reGrepMultiLn  = regexp.MustCompile(`^([^\d:\n][^:\n]*):(\d+):(.*)$`)
	reGrepMultiCtx = regexp.MustCompile(`^([^\d:\n][^:\n]*)-(\d+)-(.*)$`)

	// Single file with line numbers (grep -n on one file).
	reGrepSingleLn  = regexp.MustCompile(`^(\d+):(.*)$`)
	reGrepSingleCtx = regexp.MustCompile(`^(\d+)-(.*)$`)
)

const grepMaxCtx = 2

type grepEntry struct {
	file    string
	lineNum string
	content string
	isMatch bool
}

type grepGroup struct {
	file    string
	entries []grepEntry
}

func compressGrep(out []byte) ([]byte, bool) {
	text := strings.TrimRight(string(out), "\n")
	if text == "" {
		return nil, false
	}
	lines := strings.Split(text, "\n")

	// Detect output format from the first recognisable content line.
	hasBinary := false
	mode := ""
	for _, line := range lines {
		if line == "--" || line == "" {
			continue
		}
		if reGrepBinary.MatchString(line) {
			hasBinary = true
			continue
		}
		if reGrepMultiLn.MatchString(line) || reGrepMultiCtx.MatchString(line) {
			mode = "multi"
			break
		}
		if reGrepSingleLn.MatchString(line) || reGrepSingleCtx.MatchString(line) {
			mode = "single"
			break
		}
		break // unrecognised first content line — passthrough
	}
	if mode == "" && !hasBinary {
		return nil, false
	}

	// Parse entries.
	var binaries []string
	var allEntries []grepEntry

	for _, line := range lines {
		if line == "--" || line == "" {
			continue
		}
		if m := reGrepBinary.FindStringSubmatch(line); m != nil {
			binaries = append(binaries, m[1])
			continue
		}
		switch mode {
		case "multi":
			if m := reGrepMultiLn.FindStringSubmatch(line); m != nil {
				allEntries = append(allEntries, grepEntry{file: m[1], lineNum: m[2], content: m[3], isMatch: true})
			} else if m := reGrepMultiCtx.FindStringSubmatch(line); m != nil {
				allEntries = append(allEntries, grepEntry{file: m[1], lineNum: m[2], content: m[3], isMatch: false})
			}
		case "single":
			if m := reGrepSingleLn.FindStringSubmatch(line); m != nil {
				allEntries = append(allEntries, grepEntry{lineNum: m[1], content: m[2], isMatch: true})
			} else if m := reGrepSingleCtx.FindStringSubmatch(line); m != nil {
				allEntries = append(allEntries, grepEntry{lineNum: m[1], content: m[2], isMatch: false})
			}
		}
	}

	if len(allEntries) == 0 && len(binaries) == 0 {
		return nil, false
	}

	// Group entries by file, then limit context per group.
	var groups []grepGroup
	fileIdx := map[string]int{}
	for _, e := range allEntries {
		idx, ok := fileIdx[e.file]
		if !ok {
			idx = len(groups)
			fileIdx[e.file] = idx
			groups = append(groups, grepGroup{file: e.file})
		}
		groups[idx].entries = append(groups[idx].entries, e)
	}

	origCtxTotal, keptCtxTotal := 0, 0
	for i := range groups {
		before := groups[i].entries
		groups[i].entries = grepLimitContext(before, grepMaxCtx)
		for _, e := range before {
			if !e.isMatch {
				origCtxTotal++
			}
		}
		for _, e := range groups[i].entries {
			if !e.isMatch {
				keptCtxTotal++
			}
		}
	}

	// Decide whether to emit compressed output or passthrough.
	// Multi-file: always compress (grouping adds readability even without context).
	// Single-file: only compress when context was actually trimmed.
	// Binary-only: always compress.
	contextTrimmed := origCtxTotal != keptCtxTotal
	willCompress := mode == "multi" || len(binaries) > 0 || contextTrimmed
	if !willCompress {
		return nil, false
	}

	var b strings.Builder

	for _, bin := range binaries {
		fmt.Fprintf(&b, "[binary: %s]\n", bin)
	}

	switch mode {
	case "multi":
		for _, g := range groups {
			matchCount := 0
			for _, e := range g.entries {
				if e.isMatch {
					matchCount++
				}
			}
			if matchCount == 1 {
				fmt.Fprintf(&b, "%s (1 match)\n", g.file)
			} else {
				fmt.Fprintf(&b, "%s (%d matches)\n", g.file, matchCount)
			}
			for _, e := range g.entries {
				fmt.Fprintf(&b, "  %s: %s\n", e.lineNum, e.content)
			}
		}
	case "single":
		for _, g := range groups {
			for _, e := range g.entries {
				fmt.Fprintf(&b, "%s: %s\n", e.lineNum, e.content)
			}
		}
	}

	return []byte(b.String()), true
}

// grepLimitContext keeps at most maxCtx context lines immediately before and
// after each match line. Context lines between two close matches are shared
// (both matches can claim them); lines beyond the budget are dropped.
func grepLimitContext(entries []grepEntry, maxCtx int) []grepEntry {
	keep := make([]bool, len(entries))
	for i, e := range entries {
		if !e.isMatch {
			continue
		}
		keep[i] = true
		// Mark up to maxCtx context lines before this match.
		cnt := 0
		for j := i - 1; j >= 0 && cnt < maxCtx; j-- {
			if entries[j].isMatch {
				break
			}
			keep[j] = true
			cnt++
		}
		// Mark up to maxCtx context lines after this match.
		cnt = 0
		for j := i + 1; j < len(entries) && cnt < maxCtx; j++ {
			if entries[j].isMatch {
				break
			}
			keep[j] = true
			cnt++
		}
	}
	var out []grepEntry
	for i, e := range entries {
		if keep[i] {
			out = append(out, e)
		}
	}
	return out
}
