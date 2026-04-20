package filters

import "strings"

// DockerBuild compresses `docker build` and `docker buildx build` output.
//
// Applies to BuildKit output format (lines starting with " => "). Old-style
// docker build output (without BuildKit) contains no matching drop patterns
// and passes through unchanged.
//
// Safety invariants:
//   - Step headers (" => [N/N] ...") and CACHED lines are preserved.
//   - ERROR lines and their context blocks are preserved verbatim.
//   - The final image ID (" => => writing image ...") and tag are preserved.
//   - Internal metadata loading and transfer progress lines are dropped.
//   - If all lines are filtered out, raw output is returned unchanged.
//   - Empty input is returned as-is.
type DockerBuild struct{}

func (DockerBuild) Name() string { return "docker-build" }

func (DockerBuild) Match(cmd string, args []string) bool {
	if cmd != "docker" || len(args) == 0 {
		return false
	}
	if args[0] == "build" {
		return true
	}
	// docker buildx build …
	if args[0] == "buildx" && len(args) > 1 && args[1] == "build" {
		return true
	}
	return false
}

// dockerDropInfixList is checked with strings.Contains against each line.
// "writing image" and "naming to" are intentionally NOT listed so they are
// kept as the final artefact identifiers.
var dockerDropInfixList = []string{
	"=> [internal]",
	"=> => transferring",
	"=> => sending",
	"=> => exporting layers",
	"=> => exporting manifest",
	"=> => resolving provenance",
	"=> => writing config",
}

func (DockerBuild) Apply(stdout []byte, _ ApplyContext) []byte {
	if len(stdout) == 0 {
		return stdout
	}
	lines := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if dropDockerLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		return stdout
	}
	return []byte(strings.Join(kept, "\n") + "\n")
}

func dropDockerLine(line string) bool {
	if strings.TrimSpace(line) == "" {
		return true
	}
	for _, infix := range dockerDropInfixList {
		if strings.Contains(line, infix) {
			return true
		}
	}
	return false
}
