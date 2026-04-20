package filters

import "strings"

// PnpmInstall compresses `pnpm install` / `pnpm add` and related output.
//
// Safety invariants:
//   - Warnings (WARN) and errors (ERR_PNPM_*) are never dropped.
//   - Progress summary ("Packages:", "Progress: ... done") is preserved.
//   - Dependency tree additions/removals and section headers are dropped.
//   - If all lines are filtered out, raw output is returned unchanged.
//   - Empty input is returned as-is.
type PnpmInstall struct{}

func (PnpmInstall) Name() string { return "pnpm-install" }

func (PnpmInstall) Match(cmd string, args []string) bool {
	if cmd != "pnpm" {
		return false
	}
	if len(args) == 0 {
		return true // bare `pnpm` runs install
	}
	switch args[0] {
	case "install", "i", "add", "remove", "update", "up", "import":
		return true
	}
	return false
}

// pnpmSectionHeaders are exact section names that introduce dependency trees.
var pnpmSectionHeaders = map[string]bool{
	"dependencies:":         true,
	"devDependencies:":      true,
	"optionalDependencies:": true,
	"peerDependencies:":     true,
}

func (PnpmInstall) Apply(stdout []byte, _ ApplyContext) []byte {
	if len(stdout) == 0 {
		return stdout
	}
	lines := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if dropPnpmLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return stdout
	}
	return []byte(strings.Join(kept, "\n") + "\n")
}

func dropPnpmLine(line string) bool {
	if strings.TrimSpace(line) == "" {
		return true
	}
	// All-plus progress indicator (e.g. "+++++++++++")
	if strings.Trim(line, "+") == "" {
		return true
	}
	trimmed := strings.TrimSpace(line)
	// Dependency tree section headers
	if pnpmSectionHeaders[trimmed] {
		return true
	}
	// Dependency tree entries: "+ package version" or "- package version"
	if strings.HasPrefix(trimmed, "+ ") || strings.HasPrefix(trimmed, "- ") {
		return true
	}
	return false
}
