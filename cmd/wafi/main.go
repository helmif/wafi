// Package main is the wafi CLI entrypoint. It glues the runner, filter
// registry, stash, and ledger into a single static binary.
//
// Subcommands:
//
//	wafi run <cmd> [args...]              run a command with output filtering
//	wafi stats [--session] [--json]       show token savings
//	wafi stash list|show|clean            browse stashed raw outputs
//	wafi doctor                           check setup health
//	wafi version                          print version
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"wafi/internal/filters"
	"wafi/internal/ledger"
	"wafi/internal/runner"
	"wafi/internal/stash"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "stats":
		os.Exit(cmdStats(os.Args[2:]))
	case "stash":
		os.Exit(cmdStash(os.Args[2:]))
	case "doctor":
		os.Exit(cmdDoctor(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println(version)
	case "init":
		os.Exit(cmdInit(os.Args[2:]))
	case "hook":
		os.Exit(cmdHook(os.Args[2:]))
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "wafi: unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `wafi — CLI token-reduction proxy

Usage:
  wafi run <cmd> [args...]              Run a command with output filtering
  wafi stats [--session] [--json]       Show token savings summary
  wafi stash list                       List recently stashed outputs
  wafi stash show <id>                  Print full stashed content
  wafi stash clean [--older-than DUR] [--yes]
                                        Delete old stash files (default 7d)
  wafi init                             Register PreToolUse hook in .claude/settings.json
  wafi hook rewrite                     Rewrite Bash tool commands (hook stdin→stdout)
  wafi doctor                           Check setup health
  wafi version                          Print the wafi version
`)
}

// sessionID derives a stable session identifier from CLAUDE_SESSION_ID if
// present, otherwise from parent PID + date.
func sessionID() string {
	if s := os.Getenv("CLAUDE_SESSION_ID"); s != "" {
		return s
	}
	return fmt.Sprintf("pid%d-%s", os.Getppid(), time.Now().UTC().Format("2006-01-02"))
}

// ---- run -----------------------------------------------------------------

func cmdRun(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: wafi run <cmd> [args...]")
		return 2
	}
	name, cmdArgs := args[0], args[1:]

	result, err := runner.Run(context.Background(), name, cmdArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		if result != nil {
			return result.ExitCode
		}
		return 1
	}

	// Runner already streamed oversized output directly; nothing to filter.
	if result.Truncated {
		return result.ExitCode
	}

	filter := filters.Default().Lookup(name, cmdArgs)

	rawLen := len(result.Stdout)
	out := result.Stdout
	filterName := ""
	passthrough := true

	if filter != nil {
		filterName = filter.Name()
		passthrough = false
		filtered, ferr := filters.SafeApply(filter, result.Stdout, filters.ApplyContext{
			Cmd:      name,
			Args:     cmdArgs,
			ExitCode: result.ExitCode,
			Stderr:   result.Stderr,
		})
		if ferr != nil {
			fmt.Fprintf(os.Stderr, "[wafi] filter %s failed: %v (passthrough)\n", filterName, ferr)
		}
		out = filtered
	}
	filteredLen := len(out)

	// On failure with a filter applied, stash the raw output so the AI can
	// recover full detail if the compressed version omits something.
	if filter != nil && result.ExitCode != 0 {
		if entry, serr := stash.Save(name, result.Stdout, result.Stderr); serr == nil {
			hint := fmt.Sprintf("\n[wafi] full output: %s\n", entry.Path)
			out = append(out, []byte(hint)...)
		}
	}

	_, _ = os.Stdout.Write(out)
	_, _ = os.Stderr.Write(result.Stderr)

	if l, lerr := ledger.Load(sessionID()); lerr == nil {
		_ = l.RecordCommand(filterName, rawLen, filteredLen, passthrough)
	}

	return result.ExitCode
}

// ---- stats ---------------------------------------------------------------

func cmdStats(args []string) int {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sessionOnly := fs.Bool("session", false, "current session only")
	asJSON := fs.Bool("json", false, "machine-readable JSON output")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	l, err := ledger.Load(sessionID())
	if err != nil {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		return 1
	}
	sess := l.CurrentSession()
	lt := l.Lifetime()

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		var payload any
		if *sessionOnly {
			payload = sess
		} else {
			payload = struct {
				Session  ledger.SessionEntry  `json:"session"`
				Lifetime ledger.LifetimeStats `json:"lifetime"`
			}{sess, lt}
		}
		_ = enc.Encode(payload)
		return 0
	}

	if !*sessionOnly {
		fmt.Println("Lifetime")
		fmt.Printf("  commands filtered:     %d\n", lt.CommandsFiltered)
		fmt.Printf("  commands passthrough:  %d\n", lt.CommandsPassthrough)
		fmt.Printf("  tokens raw:            %d\n", lt.TokensRaw)
		fmt.Printf("  tokens filtered:       %d\n", lt.TokensFiltered)
		fmt.Printf("  tokens saved:          %d\n", lt.TokensSaved)
		fmt.Printf("  repeat reads blocked:  %d\n", lt.RepeatReadsBlocked)
		fmt.Println()
	}
	fmt.Printf("Session %s\n", sess.ID)
	fmt.Printf("  started:       %s\n", sess.Started.Local().Format(time.RFC3339))
	fmt.Printf("  commands:      %d\n", sess.Commands)
	fmt.Printf("  tokens saved:  %d\n", sess.TokensSaved)
	return 0
}

// ---- stash ---------------------------------------------------------------

type stashFile struct {
	id        string
	cmd       string
	timestamp time.Time
	size      int64
	path      string
}

func listStashFiles() ([]stashFile, error) {
	dir, err := stash.Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []stashFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".log")
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			continue
		}
		ts, perr := strconv.ParseInt(parts[0], 10, 64)
		if perr != nil {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		tail := parts[0]
		if len(tail) > 4 {
			tail = tail[len(tail)-4:]
		}
		out = append(out, stashFile{
			id:        fmt.Sprintf("%s-%s", tail, parts[1]),
			cmd:       parts[1],
			timestamp: time.Unix(ts, 0),
			size:      info.Size(),
			path:      filepath.Join(dir, e.Name()),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].timestamp.After(out[j].timestamp)
	})
	return out, nil
}

func cmdStash(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: wafi stash <list|show|clean> ...")
		return 2
	}
	switch args[0] {
	case "list":
		return cmdStashList(args[1:])
	case "show":
		return cmdStashShow(args[1:])
	case "clean":
		return cmdStashClean(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "wafi: unknown stash subcommand %q\n", args[0])
		return 2
	}
}

func cmdStashList(_ []string) int {
	files, err := listStashFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		fmt.Println("no stash files")
		return 0
	}
	if len(files) > 20 {
		files = files[:20]
	}
	fmt.Printf("%-22s  %-19s  %-24s  %s\n", "ID", "DATE", "COMMAND", "SIZE")
	for _, f := range files {
		fmt.Printf("%-22s  %-19s  %-24s  %s\n",
			f.id,
			f.timestamp.Local().Format("2006-01-02 15:04:05"),
			f.cmd,
			humanSize(f.size),
		)
	}
	return 0
}

func findStashFile(id string) (*stashFile, error) {
	files, err := listStashFiles()
	if err != nil {
		return nil, err
	}
	for i := range files {
		f := files[i]
		base := filepath.Base(f.path)
		if id == f.id || id == base || id == strings.TrimSuffix(base, ".log") {
			return &f, nil
		}
	}
	return nil, fmt.Errorf("no stash file matching %q", id)
}

func cmdStashShow(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: wafi stash show <id>")
		return 2
	}
	sf, err := findStashFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		return 1
	}
	data, err := os.ReadFile(sf.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		return 1
	}
	_, _ = os.Stdout.Write(data)
	return 0
}

func cmdStashClean(args []string) int {
	fs := flag.NewFlagSet("stash clean", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	olderThan := fs.String("older-than", "7d", "delete files older than this duration (e.g. 7d, 24h)")
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dur, err := parseDuration(*olderThan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wafi: invalid duration %q: %v\n", *olderThan, err)
		return 2
	}
	if !*yes {
		fmt.Printf("Delete stash files older than %s? [y/N] ", *olderThan)
		var resp string
		_, _ = fmt.Scanln(&resp)
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp != "y" && resp != "yes" {
			fmt.Println("aborted")
			return 0
		}
	}
	n, err := stash.CleanupOlderThan(dur)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		return 1
	}
	fmt.Printf("removed %d file(s)\n", n)
	return 0
}

// parseDuration extends time.ParseDuration with a "d" (day) suffix.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if head, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(head)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGT"[exp])
}

// ---- init ----------------------------------------------------------------

const (
	claudeDir      = ".claude"
	settingsFile   = ".claude/settings.json"
	hookCmd        = "wafi hook rewrite"
	hookMatcher    = "Bash"
)

func cmdInit(_ []string) int {
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		return 1
	}

	var root map[string]any
	data, err := os.ReadFile(settingsFile)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		return 1
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			fmt.Fprintf(os.Stderr, "wafi: cannot parse %s: %v\n", settingsFile, err)
			return 1
		}
	}
	if root == nil {
		root = map[string]any{}
	}

	if hookAlreadyRegistered(root) {
		fmt.Println("Already registered")
		return 0
	}

	registerHook(root)

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		return 1
	}
	out = append(out, '\n')
	if err := os.WriteFile(settingsFile, out, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "wafi: %v\n", err)
		return 1
	}
	fmt.Println("Hook registered in .claude/settings.json")
	return 0
}

// hookAlreadyRegistered reports whether a "wafi hook rewrite" entry already
// exists anywhere in the PreToolUse hook list.
func hookAlreadyRegistered(root map[string]any) bool {
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return false
	}
	preList, ok := hooks["PreToolUse"].([]any)
	if !ok {
		return false
	}
	for _, item := range preList {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		innerList, ok := m["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range innerList {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if hm["command"] == hookCmd {
				return true
			}
		}
	}
	return false
}

// registerHook adds the PreToolUse entry to root, merging with any existing hooks.
func registerHook(root map[string]any) {
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}

	entry := map[string]any{
		"matcher": hookMatcher,
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCmd,
			},
		},
	}

	preList, _ := hooks["PreToolUse"].([]any)
	hooks["PreToolUse"] = append(preList, entry)
}

// ---- hook ----------------------------------------------------------------

// knownFilteredCmds is the set of binaries whose output wafi can filter.
var knownFilteredCmds = map[string]bool{
	"git":    true,
	"npm":    true,
	"pnpm":   true,
	"yarn":   true,
	"docker": true,
	"go":     true,
	"jest":   true,
	"vitest": true,
	"cargo":  true,
	"ls":     true,
	"find":   true,
	"grep":   true,
	"diff":   true,
}

func cmdHook(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: wafi hook rewrite")
		return 2
	}
	switch args[0] {
	case "rewrite":
		return cmdHookRewrite()
	default:
		fmt.Fprintf(os.Stderr, "wafi: unknown hook subcommand %q\n", args[0])
		return 2
	}
}

func cmdHookRewrite() int {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		// Can't even read — pass nothing through; exit 0 so hook doesn't block.
		return 0
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		// Malformed input — forward as-is.
		_, _ = os.Stdout.Write(raw)
		return 0
	}

	toolInput, _ := payload["tool_input"].(map[string]any)
	if toolInput != nil {
		if cmd, ok := toolInput["command"].(string); ok {
			if rewritten, changed := rewriteCommand(cmd); changed {
				toolInput["command"] = rewritten
				payload["tool_input"] = toolInput
			}
		}
	}

	out, err := json.Marshal(payload)
	if err != nil {
		_, _ = os.Stdout.Write(raw)
		return 0
	}
	_, _ = os.Stdout.Write(out)
	return 0
}

// rewriteCommand prepends "wafi run" if the first word of cmd is a known
// filtered binary. Returns (new command, changed).
func rewriteCommand(cmd string) (string, bool) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return cmd, false
	}
	binary := strings.Fields(trimmed)[0]
	// Strip any path prefix so "/usr/bin/git" → "git".
	binary = filepath.Base(binary)
	if !knownFilteredCmds[binary] {
		return cmd, false
	}
	return "wafi run " + trimmed, true
}

// ---- doctor --------------------------------------------------------------

func cmdDoctor(_ []string) int {
	ok := true
	report := func(status, name, detail string) {
		fmt.Printf("%-4s  %-24s  %s\n", status, name, detail)
	}

	if p, err := exec.LookPath("wafi"); err != nil {
		report("WARN", "binary in PATH", err.Error())
	} else {
		report("OK", "binary in PATH", p)
	}

	if d, err := stash.Dir(); err != nil {
		report("FAIL", "stash directory", err.Error())
		ok = false
	} else if err := os.MkdirAll(d, 0o700); err != nil {
		report("FAIL", "stash directory", err.Error())
		ok = false
	} else {
		probe := filepath.Join(d, ".wafi-doctor")
		if err := os.WriteFile(probe, []byte("x"), 0o600); err != nil {
			report("FAIL", "stash directory", fmt.Sprintf("%s (not writable)", d))
			ok = false
		} else {
			_ = os.Remove(probe)
			report("OK", "stash directory", d)
		}
	}

	if l, err := ledger.Load(sessionID()); err != nil {
		report("FAIL", "ledger readable", err.Error())
		ok = false
	} else {
		report("OK", "ledger readable",
			fmt.Sprintf("commands_filtered=%d tokens_saved=%d",
				l.Lifetime().CommandsFiltered, l.Lifetime().TokensSaved))
	}

	hookPath := ".claude/settings.json"
	data, err := os.ReadFile(hookPath)
	switch {
	case err != nil:
		report("WARN", "claude code hook", fmt.Sprintf("%s not found (run `wafi init`)", hookPath))
	case !strings.Contains(string(data), "wafi"):
		report("WARN", "claude code hook", "no wafi entry in settings.json")
	default:
		report("OK", "claude code hook", hookPath)
	}

	if !ok {
		return 1
	}
	return 0
}
