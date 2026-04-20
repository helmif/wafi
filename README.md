# wafi — sufficient output, efficient tokens

wafi (وافي — Arabic for *sufficient*, *complete*, *faithful*) is a local CLI
proxy that wraps noisy shell commands and returns a compact, deterministic
summary to the tool or agent that called it. It exists because AI coding
tools spend more tokens reading command output than writing code, and most
of that output is ceremony — progress bars, hint lines, ASCII trees — that
the AI neither needs nor rereads. wafi strips the ceremony and keeps the
signal. No LLM, no network, no telemetry. Single static Go binary.

## Install

```bash
brew install helmif/tap/wafi      # coming soon
```

Until the tap is live, grab a binary from the
[releases page](https://github.com/helmif/wafi/releases) and drop it on
your `$PATH`. Prebuilt binaries ship for `linux/amd64`, `linux/arm64`,
`darwin/amd64`, `darwin/arm64`, and `windows/amd64`.

## Quick start

```bash
cd your-project
wafi init
```

`wafi init` registers a `PreToolUse` hook in `.claude/settings.json`. After
that, keep using Claude Code normally — any supported command it runs is
automatically prefixed with `wafi run`, the output is filtered, and your
token count goes down.

## Supported commands

| Command            | Dropped                                              | Kept                                                            |
| ------------------ | ---------------------------------------------------- | --------------------------------------------------------------- |
| `git status`       | hint lines, blank separators, verbose section labels | branch, upstream, per-file M/A/D/R/C/U/? status                 |
| `git diff`         | `index ` SHAs, `similarity index`, context lines     | `@@` hunks, `+`/`-` lines, file headers, mode/rename/binary     |
| `git log`          | full hash, email, GPG sig, body, `--stat` file lines | `<hash7> <subject> (<author>, <reldate>)` per commit            |
| `git push`         | `remote:` progress, object counting/compressing      | `To <remote>`, branch refs, errors/hints                        |
| `git pull`         | `remote:` progress, `Unpacking objects` lines        | `From`, fast-forward stats, `CONFLICT`, merge headlines         |
| `git branch`       | `HEAD ->` pointer, `remotes/` prefix                 | current branch `*` marker, `[remote]` prefix on remotes         |
| `npm install`      | funding notice, audit/fund hints, blank lines        | install summary, vulnerability count, errors, deprecations      |
| `pnpm install`     | `++++++` progress bars, dep tree lines               | `Packages: +N`, progress tail, warnings, `ERR_PNPM_*` errors    |
| `yarn install`     | `[N/N]` step progress, `info ` lines, tree glyphs    | success, error, warning, `Done in Xs`                           |
| `docker build`     | `=> [internal]` meta, transfer/export progress       | `=> [N/N]` step headers, `=> CACHED`, `=> ERROR`, image ID/tag  |
| `go test`          | `--- PASS:` lines                                    | `--- FAIL:` + indented context, summary line, coverage %        |
| `jest` / `vitest`  | ` PASS` / ` ✓ ` passing-file rows                    | failing-file rows, `●` / `❯` error blocks, summary block        |
| `cargo test`       | `Compiling`/`Finished`/`Running`, `test ... ok`      | `test ... FAILED` rows, `failures:` block, `test result:` line  |
| `ls -l`            | `total N`, `.`/`..`, perms, owner, group, timestamp  | names, human-readable sizes, `/` on dirs, symlink targets       |
| `find`             | `Permission denied` / `Operation not permitted`      | matched paths, truncation summary, one-line denied count        |
| `grep` / `rg`      | excess context lines beyond 2 before/after           | filename grouping, line numbers, matched text, binary notices   |
| `diff`             | unchanged context beyond 3 consecutive lines         | `@@` hunks, `+`/`-`, file headers, `***` section markers        |

Any command not in this table passes through unchanged.

## Usage

```bash
wafi stats                # token savings summary (session + lifetime)
wafi stats --json         # machine-readable
wafi stash list           # recent stashed raw outputs from failed commands
wafi stash show <id>      # full raw output for a failed filtered command
wafi stash clean --yes    # delete stashed outputs older than 7 days
wafi doctor               # check setup (binary in PATH, dirs writable, hook)
```

When a filtered command *fails*, wafi writes the raw, unfiltered output to
`~/.local/state/wafi/stash/` (or `$XDG_STATE_HOME/wafi/stash/`) and appends
the path to stdout. The AI can always recover full detail if the compressed
view dropped something relevant.

## How it works

- `wafi run <cmd>` executes `<cmd>` in a subprocess, captures stdout/stderr,
  preserves the exit code and forwards signals (`SIGINT`, `SIGTERM`).
- A registry of deterministic filters (regex/string ops) rewrites stdout.
  No LLM, no network, no file I/O beyond the stash on failure.
- A `PreToolUse` hook in `.claude/settings.json` transparently rewrites the
  agent's `Bash` tool invocations — the agent doesn't know wafi is there.

## Zero-risk guarantee

wafi is built on five non-negotiable principles:

1. **Unknown command = pass-through.** Output is unchanged.
2. **Filter failure or panic returns the raw output.** A bug in wafi never
   fails the user's command.
3. **Filters are deterministic.** Regex and string operations only — no
   LLM, no network, no disk I/O during filtering.
4. **Error messages and stack traces are preserved verbatim.** Only
   ceremony (progress bars, hint lines, blank separators) is dropped.
5. **Exit codes are propagated exactly.** `wafi run foo` exits with the
   same code `foo` would have.

When a filter must choose between *fewer tokens* and *safer*, it chooses
safer. When in doubt, the filter returns input unchanged.

## License

MIT
