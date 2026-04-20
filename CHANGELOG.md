# Changelog

All notable changes to wafi are documented here.

## [0.1.0] — 2026-04-20

### Phase 14 — docs & release (2026-04-20)

- README with install, quick start, supported-commands table, how-it-works,
  zero-risk guarantee.
- SECURITY.md documenting threat model, what stays local, what wafi never does.
- CONTRIBUTING.md with filter-addition workflow and PR checklist.
- GitHub Actions: `ci.yml` (test, vet, coverage, five-platform build) and
  `release.yml` (five-platform release build, SHA256 checksums, auto-notes
  from CHANGELOG).
- Version bumped from `0.1.0-dev` → `0.1.0`.

### Phase 13 — testing infrastructure (2026-04-20)

- Tests added for `cmd/wafi`, `internal/runner`, `internal/stash`,
  `internal/testutil`, plus broader coverage in `internal/memory`.
- All production packages ≥ 80% statement coverage. Filter package stays
  at 94.9%.
- Integration suite added under `integration/run_test.go` (build tag
  `integration`): version string, `wafi run echo`, filtered `git status`
  shorter than raw, `wafi doctor` no-panic check.
- gosec clean (four known-benign `G204`/`G304` sites annotated with
  `#nosec` + justification): runner's `exec.CommandContext` is wafi's
  entire purpose, and the three `os.ReadFile` sites use
  internally-constructed paths under XDG state dirs.
- `go vet ./...` clean.

### Phase 7 — filesystem filters (2026-04-20)

Added four filesystem filters — completes Phase 7.

**ls** (`internal/filters/ls_filter.go`)
- Matches: any `ls` invocation with a short `-l` flag (`-l`, `-la`, `-lh`, `-lR`, `-al`, etc.)
- Keeps: filenames, sizes (converted to human-readable B/K/M/G), directories marked with `/`, symlink arrows (`link -> target`)
- Drops: `total N` line, `.`/`..` entries, permissions string, link count, owner, group, timestamp
- Handles: `-h` pre-formatted sizes (pass-through), old files with year instead of time, `-lR` recursive section headers, empty dirs → `(empty)`
- Passthrough: no long-format lines detected in output
- 9 test cases (match, 6 apply variants, recursive)

**find** (`internal/filters/find_filter.go`)
- Matches: any `find` invocation
- Keeps: matched paths (up to 40 when truncating), truncation summary `... (+N more)`, permission-denied count note
- Drops: `find: ... Permission denied` / `Operation not permitted` lines (counted, reported at end)
- Passthrough: ≤50 lines with no permission errors (already compact)
- 6 test cases (match, passthrough at threshold, large truncation, denied stripping, combined, empty)

**grep / rg** (`internal/filters/grep_filter.go`)
- Matches: `grep` and `rg`
- Groups multi-file output by filename: `src/auth.go (3 matches)` header + `  N: content` per kept line
- Limits context lines to max 2 before/after each match (excess `-C`/`-A`/`-B` context dropped)
- Collapses binary-file notices to `[binary: path]`
- Passthrough: unknown format (no filename/line-number prefix, no binary), empty output, single-file with ≤2 context already
- 10 test cases (match, multi-file, context limiting, binary-only, binary mixed, no matches, single-file passthrough, single-file excess context, unknown format, `--` separator)

**diff** (`internal/filters/diff_filter.go`)
- Matches: standalone `diff` command only (`cmd=="diff"`); `git diff` (`cmd=="git"`) is handled by `GitDiff` and can never match here
- Detects format from first structural line: `@@ ` → unified, `***************` → context, neither → normal format
- Unified (`-u`): keeps `--- /+++` file headers, `@@ ` hunk headers, all `+`/`-` change lines, max 3 consecutive context lines per run; `\ No newline` annotations always kept
- Context (`-c`): keeps `*** /---` headers, `***************` separators, all `! `/ `+ `/`- ` change lines, max 3 consecutive `  ` context lines per run; resets count at each section header
- Passthrough: binary (`Binary files X and Y differ`), no differences (empty output), normal format (already compact, no context lines), diffs with ≤3 consecutive context lines (nothing to trim)
- Handles: multi-file recursive diffs (`diff -r`), `diff ` sub-headers preserved
- 10 test cases (match, unified passthrough, unified excess, multi-hunk, context format, normal passthrough, no differences, binary, no-newline annotation, multi-file)

All four filters registered in `Default()`. Phase 7 complete — 180 total tests passing.

---

### Phase 6 — test runner filters (2026-04-20)

Added four filters for test runners.

**go_test** (`internal/filters/go_test_filter.go`)
- Keeps: `--- FAIL:` lines and their full indented error context, final summary (`ok`/`FAIL`/`?`), coverage %
- Drops: `--- PASS:` lines (summary already covers pass count)
- Passthrough: `-v` flag (already compact), pure panic output (no PASS lines), no-test-files output
- 6 fixtures (4 golden, 2 passthrough)

**jest** (`internal/filters/jest_filter.go`)
- Keeps: ` FAIL  file.test.ts` lines, full `●` error blocks, summary (Test Suites / Tests / Time)
- Drops: ` PASS  file.test.ts` lines
- Passthrough: all-fail output (no PASS lines to drop); supports `jest` and `npx jest`
- 4 fixtures (3 golden, 1 passthrough)

**vitest** (`internal/filters/vitest_filter.go`)
- Keeps: ` ✗ ` failing-file lines, `❯` error blocks, summary (Test Files / Tests / Duration)
- Drops: ` ✓ ` passing-file lines
- Passthrough: all-fail output; supports `vitest` and `npx vitest`
- 3 fixtures (2 golden, 1 passthrough)

**cargo_test** (`internal/filters/cargo_test_filter.go`)
- Keeps: `test X ... FAILED` lines, full `failures:` block, `test result:` summary
- Drops: `Compiling`/`Finished`/`Running` build preamble, `running N tests` header, `test X ... ok` passing lines
- Passthrough: build errors with no test output
- 5 fixtures (4 golden, 1 passthrough)

---

### Phase 5 — package manager & docker filters (2026-04-20)

Added four filters for package managers and container builds.

**npm_install** (`internal/filters/npm_install.go`)
- Keeps: `added/changed/removed X packages` summary, `found X vulnerabilities`, `npm error`/`npm ERR!` errors, `npm warn deprecated` warnings
- Drops: funding notice, `run \`npm fund\`` hint, `To address` / `Run \`npm audit fix\`` audit hints, blank lines
- 4 fixtures

**pnpm_install** (`internal/filters/pnpm_install.go`)
- Keeps: `Packages: +N` summary, `Progress: … done`, `WARN` deprecations, `ERR_PNPM_*` errors
- Drops: `++++++` progress bars, `dependencies:` / `devDependencies:` section headers, `+ pkg version` / `- pkg version` tree entries, blank lines
- 3 fixtures

**yarn_install** (`internal/filters/yarn_install.go`)
- Classic (v1): keeps `success`, `error`, `warning`, `Done in Xs.`; drops `[N/N]` step progress, `info ` lines, `└─`/`├─`/`│` tree lines
- Berry (v2+): drops `YN0000` info lines (except `Done`), keeps all other YN codes (warnings/errors)
- 3 fixtures

**docker_build** (`internal/filters/docker_build.go`)
- Keeps: `[+] Building` summary, `=> [N/N]` step headers, `=> CACHED` lines, `=> ERROR` lines, error blocks, `=> => writing image` (final ID), `=> => naming to` (tag)
- Drops: `=> [internal]` metadata, `=> => transferring`/`sending`/`exporting layers`/`exporting manifest`/`resolving provenance`/`writing config` lines
- Old-style (non-BuildKit) output passes through unchanged
- 3 fixtures

All four filters registered in `Default()`. Filter coverage: 95.1%.

### Phase 4 — git family filters (2026-04-20)

Added five git filters completing the core git workflow coverage.

**git_diff** (`internal/filters/git_diff.go`)
- Keeps: hunk headers (`@@ `), added/removed lines (`+`/`-`), file headers (`diff --git`, `---`, `+++`), mode/rename/binary metadata
- Drops: `index ` SHA lines, `similarity index` lines, context lines (space-prefix)
- 7 fixtures, 95% filter coverage

**git_log** (`internal/filters/git_log.go`)
- Compresses verbose commit blocks to one line: `<hash7> <subject> (<author>, <reldate>)`
- Drops: full hash, email, GPG signatures, body paragraphs, blank lines, `--stat` file lines
- 14-bucket relative-date formatter; injectable `logNow` for stable golden files
- 5 fixtures

**git_push** (`internal/filters/git_push.go`)
- Keeps: `To <remote>`, branch ref update lines, error/hint lines
- Drops: `remote:` progress, counting/compressing/writing objects, delta resolution lines
- Note: `git push` writes to stderr; filter applied against representative combined output (CLI wiring in Phase 11 will merge stderr for push/pull)
- 4 fixtures

**git_pull** (`internal/filters/git_pull.go`)
- Keeps: `From`, `Already up to date.`, fast-forward arrows, stats (`X files changed`), `CONFLICT`, `Merge made by`, error lines
- Drops: `remote:` progress, `Unpacking objects` lines
- 4 fixtures

**git_branch** (`internal/filters/git_branch.go`)
- Strips `remotes/` prefix from remote-tracking branches
- Drops `HEAD ->` pointer lines (noise)
- Marks current branch with `*`; formats remotes with `[remote]` prefix
- 2 fixtures

All five filters registered in `Default()`.

### Phase 3 — git status filter (2026-04-20)

**git_status** (`internal/filters/git_status.go`)
- Compresses verbose `git status` output to compact sections
- Output: `branch: <name> (<tracking-info>)` + one-letter prefixes (M/A/D/R/C/U/?)
- 9 fixtures, 96% filter coverage
- Pass-through on any unrecognised line (zero-risk principle)

### Phase 2 — filter interface & registry (2026-04-20)

**Filter interface** (`internal/filters/filter.go`)
- `Filter` interface: `Name()`, `Match()`, `Apply()`
- `ApplyContext`: Cmd, Args, ExitCode, Stderr (read-only)
- `SafeApply`: panic-recovery wrapper — always returns raw output on filter failure

**Registry** (`internal/filters/registry.go`)
- First-match-wins dispatch
- `Default()` factory wires all built-in filters

**Test infrastructure** (`internal/testutil/golden.go`)
- Golden file helper with `-update` flag for fixture regeneration

### Phase 1 — project foundation (2026-04-20)

- `go.mod` — module `wafi`, Go 1.22, zero external dependencies
- `internal/runner/runner.go` — subprocess execution with 10 MB output cap, signal forwarding, exit code propagation
- `internal/stash/stash.go` — persist raw output on command failure (0600 files, 0700 dirs)
- Module renamed from `lean` → `wafi`
