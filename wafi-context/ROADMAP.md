# wafi — Build Roadmap

> 9 fase untuk v0.1.0. Tiap fase selesaikan satu concern, test, commit,
> baru lanjut. Jangan loncat fase.

## Fase 1: Project foundation ✅ (sudah ada)

Sudah dibuat di sesi planning sebelumnya:
- `go.mod` — module wafi, Go 1.22, zero deps
- `internal/runner/runner.go` — subprocess execution dengan safety invariants
- `internal/stash/stash.go` — persist raw output on failure
- `HANDOFF.md` — context lengkap (pindah ke `docs/CONTEXT.md`)

**Task awal:** rename module dari `lean` ke `wafi` di semua file.
Verify `go build ./...` sukses sebelum lanjut.

## Fase 2: Filter interface & registry ✅

**File:** `internal/filters/filter.go`, `internal/filters/registry.go`

**Goal:** infrastruktur untuk register dan dispatch filter.

**Filter interface:**
```go
type Filter interface {
    // Match returns true if this filter handles this command.
    // Args include the subcommand (e.g., ["status"] for "git status").
    Match(cmd string, args []string) bool

    // Apply transforms stdout. stderr is passed separately because
    // most filters should NOT touch stderr (errors must stay intact).
    Apply(stdout []byte, ctx ApplyContext) []byte

    // Name returns filter identifier for debugging / ledger.
    Name() string
}

type ApplyContext struct {
    Cmd      string
    Args     []string
    ExitCode int
    Stderr   []byte // read-only, for context only
}
```

**Registry:**
```go
type Registry struct {
    filters []Filter
}

func (r *Registry) Register(f Filter) { ... }

// Lookup returns first matching filter, or nil (pass-through).
func (r *Registry) Lookup(cmd string, args []string) Filter { ... }
```

**Zero-risk wrapper:** `SafeApply` yang recover dari panic, kembalikan raw
output kalau filter error:
```go
func SafeApply(f Filter, stdout []byte, ctx ApplyContext) (out []byte, err error) {
    defer func() {
        if r := recover(); r != nil {
            out = stdout
            err = fmt.Errorf("filter %s panicked: %v", f.Name(), r)
        }
    }()
    return f.Apply(stdout, ctx), nil
}
```

**Testing:** mock filter yang panic, pastiin SafeApply return raw + error.

## Fase 3: Filter #1 — `git status` ✅

**File:** `internal/filters/git_status.go` + `testdata/git_status_*.txt`

**Goal:** proof of concept. Satu filter yang solid dulu sebelum generalize.

**Target transformation:**
```
# Input (500 tokens)
On branch feature/auth
Your branch is up to date with 'origin/feature/auth'.

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
	modified:   src/auth.go
	modified:   src/user.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)
	src/new.go

no changes added to commit (use "git add" and/or "git commit -a")

# Output (30 tokens)
branch: feature/auth (up to date)
M src/auth.go
M src/user.go
? src/new.go
```

**Test cases di testdata/:**
- `clean.txt` — working tree clean
- `dirty.txt` — modified + untracked
- `staged.txt` — some staged, some not
- `ahead.txt` — branch ahead of remote
- `behind.txt` — branch behind
- `diverged.txt` — ahead + behind
- `detached.txt` — detached HEAD
- `conflict.txt` — merge conflict
- `malformed.txt` — garbage input → expect pass-through

**Rule:** kalau input parse-nya gagal (missing expected header, etc), return
raw input. Jangan hallucinate structure.

## Fase 4: Filter #2-6 — git family ✅

Lanjut dengan filter git lain:

- **`git_diff`** — keep hunk headers + changes, drop context lines — 7 fixtures, 95% cov ✅
- **`git_log`** — multi-line commits → one-liner per commit — 5 fixtures, injectable clock ✅
- **`git_push`** — keep "To <remote>" + summary, drop progress/remote: lines ✅
- **`git_pull`** — keep summary/fast-forward/stats, drop remote:/Unpacking ✅
- **`git_branch`** — strip remotes/ prefix, drop HEAD pointer lines ✅

Pattern yang sama: testdata fixtures, golden files, pass-through on parse
failure.

## Fase 5: Filter #7-10 — package managers ✅ DONE

- **`npm_install`** — keep: final summary (X packages, Y vulnerabilities),
  errors, deprecation warnings. Drop: progress bars, spinners, tree.
- **`pnpm_install`** — sama, adjust untuk pnpm output format
- **`yarn_install`** — sama
- **`docker_build`** — keep: step headers, errors, final image ID. Drop:
  transfer progress, cache hit lines, layer download bytes.

**Critical:** kalau ada error atau vulnerability warning, WAJIB preserved.
User butuh itu buat fix security issue.

## Fase 6: Filter #11-14 — test runners ✅ DONE

- **`go_test`** — keep: `--- FAIL:` + error context, summary. Drop: `--- PASS:`
  lines. Passthrough: `-v` flag, pure panics, no-test-files.
- **`jest`** / **`npx jest`** — keep: ` FAIL ` file lines, `●` error blocks,
  summary. Drop: ` PASS ` file lines.
- **`vitest`** / **`npx vitest`** — keep: ` ✗ ` file lines, `❯` error blocks,
  summary. Drop: ` ✓ ` passing-file lines.
- **`cargo_test`** — keep: `... FAILED` lines, failures block, `test result:`.
  Drop: Compiling/Finished/Running preamble, `... ok` passing lines.

**Critical:** failed test output HARUS literal. Jangan summarize error message.

## Fase 7: Filter #14-17 — filesystem

- **`ls`** — compact tree view untuk `ls -la` atau `ls -R`
- **`find`** — group by directory kalau results banyak
- **`grep`** — group by file, collapse adjacent lines
- **`diff`** — compact hunks, drop file header kalau obvious

## Fase 8: Anti-repeated-read

**File:** `internal/memory/memory.go`

**Goal:** track file reads per session, warn on duplicate.

**Design:**
```go
type Session struct {
    ID       string
    Started  time.Time
    Reads    map[string]ReadRecord  // path → record
}

type ReadRecord struct {
    FirstRead  time.Time
    ReadCount  int
    FileHash   string  // sha256 of content
    TokenEst   int
}
```

**Storage:** `~/.local/state/wafi/sessions/<session-id>.json`

**Session ID:** derived from `CLAUDE_SESSION_ID` env var kalau ada, else
hash of (tty + ppid + start time).

**Integration:** filter baru `ReadFileFilter` yang match `cat`, `head`,
`tail`, `less` command dengan file path arg. Before returning content:
1. Hash content
2. Check session.Reads[path]
3. Kalau belum ada → record + return normal
4. Kalau ada + hash sama → return summary:
   ```
   [wafi] File already read this session (3 times total, ~450 tokens).
   Content unchanged since first read. If you really need to re-read,
   use: cat --fresh <file>
   ```
5. Kalau ada + hash beda → return full content + note:
   ```
   [wafi] File previously read this session. Content has changed.
   ```

**Escape hatch:** env var `WAFI_DISABLE_DEDUP=1` untuk bypass.

## Fase 9: Token ledger

**File:** `internal/ledger/ledger.go`

**Goal:** track savings, viewable via CLI.

**Schema:**
```json
{
  "version": 1,
  "lifetime": {
    "commands_filtered": 0,
    "commands_passthrough": 0,
    "input_tokens_raw": 0,
    "input_tokens_filtered": 0,
    "tokens_saved": 0,
    "repeat_reads_blocked": 0
  },
  "sessions": [
    {
      "id": "...",
      "started": "2026-04-20T10:00:00Z",
      "commands": 42,
      "tokens_saved": 15420
    }
  ]
}
```

**Token estimation:** simple char/4 heuristic (tiktoken ratio avg ~4 chars
per token untuk English + code). Document prominently bahwa ini estimate,
bukan exact.

**Storage:** `~/.local/state/wafi/ledger.json`

**CLI:**
```
wafi stats                  # lifetime summary
wafi stats --session        # current session only
wafi stats --json           # machine-readable
```

**Safety:** ledger write pake atomic rename (`ledger.json.tmp` → rename).
Kalau corrupt, skip update dan log warning (jangan crash).

## Fase 10: Stash viewer

**File:** `cmd/wafi/stash.go`

**Goal:** CLI buat browse stash files.

**Commands:**
```
wafi stash list             # list recent, format: ID | date | cmd | size
wafi stash show <id>        # print full raw output
wafi stash clean [--older-than 7d]  # delete old
wafi stash path <id>        # print just the path (for scripting)
```

**ID format:** pake timestamp dari filename. User-friendly short form
(last 4 digits timestamp + cmd name).

**Safety:** `stash clean` konfirmasi (y/N) kecuali `--yes` flag.

## Fase 11: CLI entrypoint

**File:** `cmd/wafi/main.go`

**Goal:** glue semua jadi satu binary.

**Commands:**
```
wafi run <cmd> [args...]    # main entry: execute + filter
wafi init                   # generate Claude Code hook config
wafi stats [flags]
wafi stash <subcommand>
wafi version
wafi doctor                 # check setup health
```

**No flags parsing library.** Pake `flag` stdlib. Kalau butuh subcommand
dispatch, bikin sendiri (30 lines of code max).

## Fase 12: Hook integration

**File:** `cmd/wafi/init.go` + `internal/hook/claude_code.go`

**Goal:** `wafi init` generate working hook config untuk Claude Code.

**Output:** edit (atau create) `.claude/settings.json` dengan:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "wafi hook rewrite"
          }
        ]
      }
    ]
  }
}
```

**`wafi hook rewrite`** — subcommand yang:
1. Read JSON from stdin (Claude Code hook protocol)
2. Parse tool_input.command
3. Kalau command ada di known list → rewrite jadi `wafi run <cmd>`
4. Return modified JSON ke stdout

**Safety:** kalau hook crash atau timeout, Claude Code fallback ke raw
command. Pastiin hook NEVER block command user.

**Idempotent:** run `wafi init` 2x ga double-register hook.

## Fase 13: Testing infrastructure

**Goal:** solid test foundation sebelum release.

- `internal/filters/*_test.go` — unit test per filter
- `internal/runner/runner_test.go` — integration test dengan real subprocess
- `testdata/` — fixture files per filter
- `integration/` — end-to-end test yang jalanin binary di temp project
- Golden file testing helper di `internal/testutil/`

**Coverage check di CI:** fail build kalau filter coverage <80%.

## Fase 14: Docs & release

- `README.md` — what, why, install, usage
- `SECURITY.md` — threat model, responsible disclosure
- `CONTRIBUTING.md` — how to add new filter
- `CHANGELOG.md` — version history
- `.github/workflows/release.yml` — build binaries, checksum, attach
- `.github/workflows/ci.yml` — test + lint + build on PR
- SHA256 checksums di release notes
- Optional: homebrew formula, arch AUR, Nix package

**Tag v0.1.0 setelah semua checklist selesai.**

---

## Cara kerja di Claude Code

Per fase:
1. Claude Code baca CONTEXT.md + ROADMAP.md + fase yang dikerjain
2. Implement + test
3. Run `go test ./... && go vet ./...`
4. Commit dengan message jelas
5. STOP. Report ke lu. Tunggu approval buat lanjut.

Jangan skip fase. Jangan kerjain multi-fase sekaligus. Incremental = reviewable.
