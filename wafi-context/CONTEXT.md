# wafi — Project Context

> Untuk Claude Code: baca dokumen ini dulu sebelum nulis kode apa pun. Ini
> source of truth untuk apa yang dibuild, kenapa, dan apa yang tidak boleh.

## Apa itu wafi

`wafi` adalah CLI token-reduction tool untuk AI coding agents (Claude Code,
Cursor, Copilot CLI, Gemini CLI, Codex). Dia intercept shell command yang
dijalanin AI, filter noise dari output, balikin hasil ringkas ke AI.

Nama dari Arabic وافي (*wāfī*) = "sufficient, adequate, complete". Filosofi
tool: keep yang **cukup**. Bukan buang semua, bukan keep semua.

## Kenapa build ini

Existing tools (rtk, omni, openwolf) punya masalah:
- **rtk** — paling solid, tapi author + audit trail-nya belum bisa di-verify
- **omni** — scope terlalu broad, bisa over-filter
- **openwolf** — terlalu banyak moving parts, depend on AI compliance
- Semua — binary yang kita ga control. Trust issue.

Solusi: build sendiri, audit sendiri, open source, reproducible build.

## Target user

Developer yang pake AI coding agent daily dan:
1. Peduli sama cost token (Anthropic/OpenAI bill bulanan mahal)
2. Peduli sama security (ga mau binary random intercept shell mereka)
3. Mau tool yang predictable (deterministic > AI-based)

## v0.1.0 scope (locked)

Empat feature, semua wajib ada di first release:

### 1. Shell output filtering
Intercept command umum dev, filter noise, return compressed output.
Commands yang di-cover: `git`, `npm`, `pnpm`, `yarn`, `docker`, `cargo`,
`go test`, `jest`, `vitest`, `ls`, `find`, `grep`, `diff`.

### 2. Anti-repeated-read
Track file yang udah dibaca AI dalam satu session. Kalau AI mau baca
file yang sama lagi, kasih warning + summary (bukan full content).

### 3. Token ledger
Track token saving per-command, per-session, lifetime. JSON file,
viewable via `wafi stats`.

### 4. Stash viewer
Kalau filter applied dan command fail, raw output disimpen ke stash.
`wafi stash list` / `wafi stash show <id>` / `wafi stash clean`.

## Yang TIDAK di v0.1.0 (intentional)

- Context pre-generator (kaya codesight) — beda kategori, bisa di v0.2
- Semantic search / RAG — butuh embeddings, complexity jump
- Learning memory / preferences file — depend on AI compliance, fragile
- Output compression (kaya caveman) — risk ke output quality
- GUI / web dashboard — CLI first, visual nanti

## Arsitektur high-level

```
┌─────────────────┐
│  Claude Code    │
│  (AI agent)     │
└────────┬────────┘
         │ mau jalanin: "git status"
         ▼
┌─────────────────┐
│ PreToolUse hook │  ← rewrite Bash tool call
│ (.claude/       │
│ settings.json)  │
└────────┬────────┘
         │ jadi: "wafi run git status"
         ▼
┌─────────────────────────────────────┐
│          wafi CLI                   │
│                                     │
│  ┌─────────┐   ┌─────────┐          │
│  │ runner  │──▶│filter   │          │
│  │ (exec)  │   │registry │          │
│  └─────────┘   └────┬────┘          │
│                     │               │
│              ┌──────┴──────┐        │
│              ▼             ▼        │
│         ┌─────────┐   ┌─────────┐   │
│         │ filter  │   │ stash   │   │
│         │ (compr) │   │ (raw)   │   │
│         └────┬────┘   └─────────┘   │
│              │                      │
│              ▼                      │
│         ┌─────────┐                 │
│         │ ledger  │                 │
│         │ (count) │                 │
│         └────┬────┘                 │
└──────────────┼──────────────────────┘
               ▼
         filtered output
               │
               ▼
          Claude Code
```

## 5 prinsip zero-risk (non-negotiable)

1. **Unknown command = pass-through.** Command tanpa filter di registry
   dijalanin dan output-nya ga diubah sedikitpun.

2. **Filter failure = pass-through.** Kalau filter regex error atau panic,
   recover dan return raw output. Jangan gagalin command user.

3. **Exit code always preserved.** Original `git push` exit 1 → `wafi run
   git push` exit 1. Ga boleh exit 0 walau filter sukses.

4. **Errors & stack traces never filtered.** Yang dibuang cuma progress,
   spinner, redundant. Error message, file paths, line numbers, stack
   traces — literal pass-through.

5. **Fallback always available.** Kalau command fail dan filter diapply,
   save raw ke stash, include path di filtered output:
   `[full output: ~/.local/state/wafi/stash/1234_git_push.log]`

## Security principles

1. **Zero external dependencies.** Cuma Go stdlib. Tiap dep = attack surface.
2. **No `sh -c` atau shell eval.** `exec.Command` langsung, args as-is.
3. **File permissions strict.** Stash files 0600, dirs 0700.
4. **Env variables ga di-log.** Bisa ada secret (API keys, tokens).
5. **No network calls.** Tool ini 100% local. Zero telemetry.
6. **Config parsing stdlib only.** `encoding/json`, `encoding/toml` lewat
   `github.com/BurntSushi/toml` kalau perlu — tapi prefer JSON biar zero dep.
7. **Path traversal defense.** Semua user-supplied path di-sanitize via
   `filepath.Clean` + check relative.
8. **Stash cleanup.** Stale stash files (>7 days default) auto-cleanable
   via `wafi stash clean`. Ga auto-delete tanpa user action.

## Testing requirements

Setiap filter WAJIB punya:
- Unit test dengan fixture file di `testdata/`
- Negative test: kasih garbage input, pastiin ga panic, return raw
- Golden file test: expected output di-commit, compare byte-by-byte

Coverage target: >80% untuk `internal/filters`, >70% overall.

## Performance requirements

- `wafi run <fast-command>` overhead harus <15ms (measured via
  `time wafi run git status` vs `time git status`)
- Memory usage untuk filter <50MB even for 10MB output
- Startup time <10ms (no heavy init)

## Release criteria v0.1.0

- [ ] Semua 4 feature working dan di-test
- [ ] Coverage >80% untuk filter package
- [ ] Cross-compile untuk: linux-amd64, linux-arm64, darwin-amd64,
      darwin-arm64, windows-amd64
- [ ] README.md lengkap dengan install, usage, contributing
- [ ] SECURITY.md dengan threat model + disclosure policy
- [ ] CI (GitHub Actions) yang run test + lint + build di tiap PR
- [ ] Checksum SHA256 buat tiap binary di release page
