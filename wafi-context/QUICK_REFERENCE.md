# wafi — Quick Reference

Satu halaman cheat sheet buat lu sendiri. Pin di sebelah editor.

## Nama & tagline

```
wafi — sufficient output, efficient tokens
(وافي: "sufficient, adequate, complete" — Arabic)
```

## Elevator pitch

CLI yang intercept shell command dari AI coding agent, filter noise,
return compressed output. Deterministic rules (no LLM). Zero deps.
Single binary. Audited by author.

## Scope v0.1.0

1. Shell output filtering (git, npm, docker, test runners, fs tools)
2. Anti-repeated-read (session-scoped file cache)
3. Token ledger (visibility)
4. Stash viewer (fallback access ke raw)

## 5 Zero-Risk Principles (hafalin)

1. Unknown command → pass-through
2. Filter panic → pass-through + log
3. Exit code always preserved
4. Errors never filtered
5. Fallback via stash always available

## 14 Fase (high level)

1. ✅ Foundation (runner, stash, go.mod, module renamed to wafi) — DONE
2. ✅ Filter interface & registry (Filter, ApplyContext, SafeApply, 100% cov) — DONE
3. ✅ git_status — 9 fixtures, 96% filter coverage — DONE
4. ✅ git family — diff, log, push, pull, branch — DONE
   - git_diff: context lines dropped, 7 fixtures, 95% cov
   - git_log: one-liner per commit, injectable clock, 5 fixtures
   - git_push: drop progress/remote:, keep To/status/errors
   - git_pull: drop remote:/Unpacking, keep From/Fast-forward/stats
   - git_branch: strip remotes/ prefix, drop HEAD pointer lines
5. ✅ Package managers (npm, pnpm, yarn, docker) — DONE
6. ✅ Test runners (go test, jest, vitest, cargo test) — DONE
7. Filesystem (ls, find, grep, diff)
8. Anti-repeated-read
9. Token ledger
10. Stash viewer
11. CLI entrypoint
12. Hook integration
13. Testing infrastructure
14. Docs & release

## Commands yang akan support (target)

```
Git:         status, log, diff, push, pull, branch, show
Package:     npm i, pnpm i, yarn, docker build
Tests:       jest, vitest, go test, cargo test
FS:          ls, find, grep, diff
```

## File structure

```
wafi/
├── cmd/wafi/                 # CLI entry
├── internal/
│   ├── runner/               # subprocess exec
│   ├── stash/                # raw output storage
│   ├── filters/              # per-command rules
│   ├── memory/               # anti-repeat
│   ├── ledger/               # token tracking
│   └── hook/                 # Claude Code integration
├── testdata/                 # fixtures
├── integration/              # e2e tests
├── docs/                     # long-form docs
├── CONTEXT.md                # this project
├── ROADMAP.md                # build plan
├── RULES.md                  # dev rules
├── README.md                 # user-facing
├── SECURITY.md               # threat model
└── CHANGELOG.md              # version history
```

## Config locations (XDG-compliant)

```
~/.config/wafi/config.json           # user config
~/.local/state/wafi/ledger.json      # token counts
~/.local/state/wafi/sessions/*.json  # session data
~/.local/state/wafi/stash/*.log      # raw fallbacks
```

## Tech stack (locked)

- **Language:** Go 1.22
- **Deps:** stdlib only
- **Test:** `testing` + golden files
- **Build:** `go build` + cross-compile via `GOOS`/`GOARCH`
- **CI:** GitHub Actions
- **Release:** GitHub Releases with SHA256 checksums

## Performance budget

- Startup: <10ms
- Overhead per command: <15ms
- Memory: <50MB even for 10MB output

## Success metrics (post-release)

- 60%+ token reduction on common workflows (measured via ledger)
- Zero data loss reports (full output always recoverable)
- Build reproducible (same commit = same binary hash)
- Install to first use <60 seconds

## Useful commands for dev

```bash
# Build & test loop
go build ./... && go test ./... && go vet ./...

# Run with verbose filter debug
WAFI_DEBUG=1 ./wafi run git status

# Update golden files
go test ./internal/filters/ -update

# Coverage
go test -coverprofile=cov.out ./... && go tool cover -html=cov.out

# Cross-compile
GOOS=darwin GOARCH=arm64 go build -o dist/wafi-darwin-arm64 ./cmd/wafi
GOOS=linux  GOARCH=amd64 go build -o dist/wafi-linux-amd64  ./cmd/wafi

# Security scan
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec ./...
```

## Kalau stuck, re-baca

1. `CONTEXT.md` — apakah approach masih align dengan goal?
2. 5 zero-risk principles — apakah compromise salah satu?
3. Dari sesi planning: rtk/omni/openwolf comparison — ambil yang kerja
