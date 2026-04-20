```
╻ ╻┏━┓┏━╸╻
┃╻┃┣━┫┣╸ ┃
┗┻┛╹ ╹╹  ┗
```

> sufficient output. efficient tokens.
> وافي — Arabic for "sufficient, adequate, complete"

![Go](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-green)
![Release](https://img.shields.io/github/v/release/helmif/wafi)

wafi sits between Claude Code and your shell. It filters noise from command output before it reaches the AI — deterministically, locally, with zero risk.

---

## Before / After

**Before** — `git log` (raw, ~400 tokens)
```
commit e409609f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d
Author: Helmi Fauzi <helmi@example.com>
Date:   Sun Apr 20 10:00:00 2026 +0700

    phase-4: git diff filter (context lines dropped, headers preserved)

commit 277e73e2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e
Author: Helmi Fauzi <helmi@example.com>
Date:   Sat Apr 19 15:30:00 2026 +0700

    phase-3: git_status fixtures, golden tests, 96% filter coverage
```

**After** — `wafi run git log` (~30 tokens)
```
e409609 phase-4: git diff filter (context lines dropped, headers preserved) (helmif, 1 day ago)
277e73e phase-3: git_status fixtures, golden tests, 96% filter coverage (helmif, 2 days ago)
```

---

**Before** — `git status` (raw, ~120 tokens)
```
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
```

**After** — `wafi run git status` (~20 tokens)
```
branch: feature/auth
upstream: origin/feature/auth (clean)
unstaged:
  M src/auth.go
  M src/user.go
untracked:
  src/new.go
```

---

## Install

### macOS

```bash
# Homebrew (recommended)
brew tap helmif/tap
brew install helmif/tap/wafi

# Apple Silicon — direct
curl -L https://github.com/helmif/wafi/releases/download/v0.1.0/wafi-darwin-arm64 -o wafi
chmod +x wafi && sudo mv wafi /usr/local/bin/

# Intel — direct
curl -L https://github.com/helmif/wafi/releases/download/v0.1.0/wafi-darwin-amd64 -o wafi
chmod +x wafi && sudo mv wafi /usr/local/bin/
```

### Linux

```bash
# AMD64
curl -L https://github.com/helmif/wafi/releases/download/v0.1.0/wafi-linux-amd64 -o wafi
chmod +x wafi && sudo mv wafi /usr/local/bin/

# ARM64
curl -L https://github.com/helmif/wafi/releases/download/v0.1.0/wafi-linux-arm64 -o wafi
chmod +x wafi && sudo mv wafi /usr/local/bin/
```

### Windows

Download `wafi-windows-amd64.exe` from the [releases page](https://github.com/helmif/wafi/releases/latest).
Rename to `wafi.exe`, add to PATH.

### Verify

```bash
wafi version
```

---

## Quick start

```bash
cd your-project
wafi init
# Hook registered in .claude/settings.json
# That's it. Use Claude Code normally.
```

---

## Supported commands

| Tool | Commands | Dropped | Kept |
|------|----------|---------|------|
| 🔧 Git | status, diff, log, push, pull, branch | progress, verbose headers | branch info, change lines, summary |
| 📦 Packages | npm/pnpm/yarn install | progress bars, tree output | summary, errors, warnings |
| 🐳 Docker | build | layer transfers, `[internal]` metadata | step headers, errors, image ID |
| 🧪 Tests | go test, jest, vitest, cargo test | passing test names | failures + full error context, summary |
| 📁 Filesystem | ls, find, grep, diff | permissions, excess context | filenames, sizes, matches |

Any command not in this table passes through unchanged.

---

## CLI reference

```bash
wafi run <cmd> [args]     # execute + filter
wafi stats                # token savings summary
wafi stash list           # browse saved raw outputs
wafi stash show <id>      # view full raw output
wafi stash clean          # delete old stash files
wafi doctor               # health check
wafi init                 # register Claude Code hook
wafi version              # version info
```

---

## wafi stats sample output

```
$ wafi stats
Lifetime
  commands filtered:    47
  commands passthrough: 3
  tokens raw:           18,420
  tokens filtered:      2,103
  tokens saved:         16,317
  repeat reads blocked: 8
```

---

## Zero-risk guarantee

> **Unknown command?** Pass-through unchanged.
> **Filter error?** Pass-through + log.
> **Command fails?** Full output saved, path shown to AI.
> **Exit code** always preserved.
> **Errors & stack traces** never filtered.

---

## How it works

1. `wafi init` registers a Claude Code hook in `.claude/settings.json`
2. Hook rewrites `git status` → `wafi run git status` transparently
3. wafi executes the command, captures output, applies filter
4. Compressed output reaches Claude Code instead of raw noise

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

## License

MIT © Helmi Fauzi
