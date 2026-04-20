# wafi

CLI token-reduction tool for Claude Code / Cursor / etc. Wraps shell
commands, filters noisy output, returns compact result to the AI.
Deterministic rules only — no LLM, no network. Single static Go binary,
stdlib only.

## Entry point flow

```
Claude → `wafi run <cmd> [args]`
      → runner.Run executes subprocess (exit code + signals preserved)
      → filters.Registry.Lookup(cmd, args)
            └─ match → SafeApply(filter, stdout, ctx)  (deterministic)
            └─ no match → pass-through
      → if exit != 0 AND filter applied: stash.Save(raw)  (path to AI)
      → write to os.Stdout, os.Exit(original_exit_code)
```

## 5 zero-risk principles (non-negotiable)

1. Unknown command = pass-through. Output unchanged.
2. Filter failure or panic → SafeApply returns raw output. Never fail
   the user's command.
3. Filter deterministic (regex/string ops). No LLM, no network, no I/O.
4. Error messages & stack traces preserved verbatim. Only ceremony
   (progress bars, hint lines, blank lines) is dropped.
5. Exit code propagated exactly.

When a filter must choose between "fewer tokens" vs "safer" → safer.
Uncertain filter → return input unchanged.

## Layout

```
cmd/wafi/main.go                 TODO — CLI entry (run / init / stash / stats)
internal/runner/runner.go        DONE — subprocess exec, 10MB cap, signals
internal/stash/stash.go          DONE — persist raw output on failure (0600)
internal/filters/
  filter.go                      DONE — Filter interface, ApplyContext, SafeApply
  registry.go                    DONE — Registry + Default()
  git_status.go                  DONE — `git status` → compact sections
  git_status_test.go             DONE — 9 fixture tests + passthrough cases
  (next)                         TODO — git log/diff/push, npm install, tests
internal/testutil/
  golden.go                      DONE — golden file helper (-update flag)
internal/config/                 TODO — ~/.config/wafi/config.json loader
```

## Filter contract

```go
type ApplyContext struct {
    Cmd      string
    Args     []string
    ExitCode int
    Stderr   []byte // read-only; filters must NOT modify stderr
}

type Filter interface {
    Name() string
    Match(cmd string, args []string) bool
    Apply(stdout []byte, ctx ApplyContext) []byte // return input if unsure
}
```

- `Match` cheap, no side effects, no mutation on args.
- `Apply` strict: if input does not match expected grammar → return bytes
  original. "When in doubt, pass-through."
- Dispatch exclusively via `SafeApply` — panic recovery built in.
- Register specific filters before general ones (first match wins).

## Build & test

```bash
go build ./...
go test ./...
go test ./internal/filters/ -update   # regenerate golden files

# cross-compile:
GOOS=darwin GOARCH=arm64 go build -o wafi-darwin-arm64 ./cmd/wafi
GOOS=linux  GOARCH=amd64 go build -o wafi-linux-amd64  ./cmd/wafi
```

## Conventions

- Zero external deps. Each dep = attack surface.
- File perm 0600, dir 0700 for everything written to disk.
- No `sh -c`; use `exec.Command` directly.
- Env variables are never logged or stashed (may contain secrets).
- User-influenced paths always sanitized (see `safeName` in stash.go).

## Deeper context

`HANDOFF.md` — planning notes from the initial session: decision log,
next phases (CLI entry, hook generator, testing, cross-compile), and
security checklist.
