package filters

import "strings"

// CargoTest compresses `cargo test` output by dropping build preamble and
// passing-test lines. FAILED lines and their full failure blocks are kept.
type CargoTest struct{}

func (CargoTest) Name() string { return "cargo-test" }

func (CargoTest) Match(cmd string, args []string) bool {
	return cmd == "cargo" && len(args) >= 1 && args[0] == "test"
}

func (CargoTest) Apply(out []byte, _ ApplyContext) []byte {
	compact, ok := compressCargoTest(out)
	if !ok {
		return out
	}
	return compact
}

func compressCargoTest(out []byte) ([]byte, bool) {
	text := string(out)
	lines := strings.Split(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	hasDroppable := false
	for _, line := range lines {
		if dropCargoLine(line) {
			hasDroppable = true
			break
		}
	}
	if !hasDroppable {
		return nil, false
	}

	var b strings.Builder
	for _, line := range lines {
		if dropCargoLine(line) {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), true
}

func dropCargoLine(line string) bool {
	// Build preamble
	if strings.HasPrefix(line, "   Compiling ") {
		return true
	}
	if strings.HasPrefix(line, "    Finished ") {
		return true
	}
	if strings.HasPrefix(line, "     Running ") {
		return true
	}
	// "running N test(s)" header
	if strings.HasPrefix(line, "running ") &&
		(strings.HasSuffix(line, " tests") || strings.HasSuffix(line, " test")) {
		return true
	}
	// Individual passing test lines: "test foo ... ok"
	if strings.HasPrefix(line, "test ") && strings.HasSuffix(line, " ... ok") {
		return true
	}
	return false
}
