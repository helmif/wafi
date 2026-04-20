# Changelog

All notable changes to wafi are documented here.

## [Unreleased] — v0.1.0

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
