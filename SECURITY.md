# Security

## What wafi does

wafi is a local shell-command proxy. It spawns a subprocess, captures that
subprocess's stdout, runs a deterministic filter (regex and string operations)
to drop known noise from the output, and writes the result to its own stdout.
The exit code, stderr, and signal behavior of the subprocess are preserved
exactly.

## What stays local

Everything. wafi has no network code. It does not phone home, does not send
telemetry, does not fetch plugins, does not call out to any LLM or external
API. The only files wafi writes to disk are:

- `$XDG_STATE_HOME/wafi/stash/` (default `~/.local/state/wafi/stash/`) —
  raw stdout/stderr from *failed* filtered commands, so the AI can recover
  full detail on demand. Files are mode `0600`; the directory is `0700`.
- `$XDG_STATE_HOME/wafi/ledger.json` — token-savings counters for
  `wafi stats`. No command content; just totals.
- `$XDG_STATE_HOME/wafi/sessions/<id>.json` — per-session file-read tracker
  used to detect redundant reads. Stores file paths and content hashes,
  never file contents.
- `.claude/settings.json` in the project dir, *only when you run `wafi init`*.

## What wafi never does

- Execute arbitrary code other than the command line you pass to `wafi run`.
- Shell out via `sh -c` or any other shell — all subprocesses are launched
  with `exec.Command` and arguments are passed directly (no word splitting,
  no glob expansion inside wafi).
- Modify environment variables before passing them to the subprocess.
- Log, stash, or transmit environment variables (they may contain secrets).
- Open network sockets or resolve hostnames.
- Call any external API, LLM, or service.
- Depend on any third-party Go module. The dependency tree is the Go
  standard library only.

## Threat model

**Protected against:**

- A buggy filter corrupting your command output. Filters are run through
  `SafeApply`, a panic-recovery wrapper. On any panic or error, the raw
  output is returned unchanged.
- A filter falsely claiming a command succeeded when it didn't. Exit codes
  are passed through verbatim; the filter never touches them.
- An attacker-controlled output attempting to crash wafi via malformed
  input. Filters are deterministic regex/string ops with bounded work; on
  any parse failure, the filter returns the input unchanged.
- Accidental writes outside the stash directory. Stash filenames are
  derived from a sanitized command name and a timestamp — no user-supplied
  path component flows into the filesystem path.
- Accidental exposure of stashed data. Files are `0600` and live under the
  user's XDG state directory.

**Not protected against:**

- A compromised command you *chose* to run. `wafi run rm -rf /` deletes
  files just as fast as `rm -rf /` does. wafi is a transparent proxy,
  not a sandbox.
- A compromised `$PATH`. wafi resolves binaries via the standard `exec`
  lookup; a malicious binary earlier in `$PATH` will be run.
- The usual risks of running an AI agent with shell access. wafi reduces
  the token cost of agent-driven shell usage; it does not vet what the
  agent chooses to run.

## Responsible disclosure

If you find a security issue, please open a GitHub issue titled
`[SECURITY]` with a description and a proof-of-concept if you have one.
Do not include secrets or credentials in the report.
