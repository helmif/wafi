package filters

import "strings"

// NpmInstall compresses `npm install` / `npm i` / `npm ci` and related output.
//
// Safety invariants:
//   - Errors (npm error, npm ERR!) are never dropped.
//   - Deprecation warnings (npm warn deprecated, npm WARN deprecated) are never dropped.
//   - Vulnerability counts are never dropped.
//   - If all lines are filtered out, raw output is returned unchanged.
//   - Empty input is returned as-is.
type NpmInstall struct{}

func (NpmInstall) Name() string { return "npm-install" }

func (NpmInstall) Match(cmd string, args []string) bool {
	if cmd != "npm" || len(args) == 0 {
		return false
	}
	switch args[0] {
	case "install", "i", "ci", "add", "remove", "uninstall", "update", "up", "upgrade":
		return true
	}
	return false
}

// npmDropSubstrings is checked with strings.Contains.
var npmDropSubstrings = []string{
	"packages are looking for funding",
	"run `npm fund`",
}

// npmDropPrefixes is checked against the trimmed line with strings.HasPrefix.
var npmDropPrefixes = []string{
	"To address",
	"Run `npm audit fix",
	"npm audit fix",
}

func (NpmInstall) Apply(stdout []byte, _ ApplyContext) []byte {
	if len(stdout) == 0 {
		return stdout
	}
	lines := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if dropNpmLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return stdout
	}
	return []byte(strings.Join(kept, "\n") + "\n")
}

func dropNpmLine(line string) bool {
	if strings.TrimSpace(line) == "" {
		return true
	}
	for _, s := range npmDropSubstrings {
		if strings.Contains(line, s) {
			return true
		}
	}
	trimmed := strings.TrimSpace(line)
	for _, p := range npmDropPrefixes {
		if strings.HasPrefix(trimmed, p) {
			return true
		}
	}
	return false
}
