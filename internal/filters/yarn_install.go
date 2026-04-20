package filters

import (
	"regexp"
	"strings"
)

// YarnInstall compresses `yarn install` / `yarn add` and related output for
// both Yarn Classic (v1) and Yarn Berry (v2+).
//
// Safety invariants:
//   - error/warning lines (Classic) are never dropped.
//   - Yarn Berry non-info codes (YN0001, YN0002, …) are never dropped.
//   - The final "Done in Xs." (Classic) or "Done in Xs Yms" (Berry) is kept.
//   - If all lines are filtered out, raw output is returned unchanged.
//   - Empty input is returned as-is.
type YarnInstall struct{}

func (YarnInstall) Name() string { return "yarn-install" }

func (YarnInstall) Match(cmd string, args []string) bool {
	if cmd != "yarn" {
		return false
	}
	if len(args) == 0 {
		return true // bare `yarn` runs install
	}
	switch args[0] {
	case "install", "add", "remove", "upgrade", "up":
		return true
	}
	return false
}

// yarnClassicStepRe matches "[1/4] Resolving packages..." step progress lines.
var yarnClassicStepRe = regexp.MustCompile(`^\[\d+/\d+\]`)

func (YarnInstall) Apply(stdout []byte, _ ApplyContext) []byte {
	if len(stdout) == 0 {
		return stdout
	}
	lines := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if dropYarnLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return stdout
	}
	return []byte(strings.Join(kept, "\n") + "\n")
}

func dropYarnLine(line string) bool {
	if strings.TrimSpace(line) == "" {
		return true
	}
	// Classic: step progress lines like "[1/4] Resolving packages..."
	if yarnClassicStepRe.MatchString(line) {
		return true
	}
	trimmed := strings.TrimSpace(line)
	// Classic: info lines (e.g. "info Direct dependencies", "info All dependencies")
	if strings.HasPrefix(trimmed, "info ") {
		return true
	}
	// Classic: dependency tree lines
	if strings.HasPrefix(trimmed, "└─") || strings.HasPrefix(trimmed, "├─") || strings.HasPrefix(trimmed, "│") {
		return true
	}
	// Berry: YN0000 is the "info" code — drop unless it marks the final "Done"
	if strings.Contains(line, "YN0000") && !strings.Contains(line, "Done") {
		return true
	}
	return false
}
