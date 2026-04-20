# Kickoff Prompt untuk Claude Code

> Copy-paste ini di message pertama Claude Code session. Attach juga
> 3 file: CONTEXT.md, ROADMAP.md, RULES.md + zip lean-starter dari
> sesi planning.

---

Gue lagi build `wafi` — CLI token-reduction tool untuk AI coding agents
(Claude Code, Cursor, Copilot CLI, dll). Tujuannya replicate core value
`rtk` dengan code yang gue audit sendiri, zero external deps, open source.

Nama dari Arabic وافي = "sufficient, adequate". Filosofi: keep yang cukup,
bukan buang semua, bukan keep semua.

## Konteks yang udah ada

Dari sesi planning sebelumnya, udah dibuat:
- Struktur project `lean/` (nama lama — rename ke `wafi`)
- `go.mod` — module definition
- `internal/runner/runner.go` — subprocess execution
- `internal/stash/stash.go` — raw output persistence

File-file itu udah di-review secara statik tapi belum di-compile. Task
pertama: rename module + verify build.

## Yang WAJIB lu lakuin sekarang

1. **Baca 3 dokumen ini dulu, dalam urutan:**
   - `CONTEXT.md` — apa yang dibuild, kenapa, prinsip zero-risk
   - `ROADMAP.md` — 14 fase untuk v0.1.0
   - `RULES.md` — aturan workflow, code, testing, security

2. **Extract zip, pindahin file ke struktur project baru.**
   Kalau nama folder masih `lean`, rename ke `wafi`. Update module
   name di `go.mod` dan semua import path.

3. **Jalanin:**
   ```bash
   go build ./... && go vet ./... && go test ./...
   ```
   Kalau ada error, fix dulu sebelum lanjut. Report hasilnya.

4. **Mulai Fase 2 (Filter interface & registry).**
   File target: `internal/filters/filter.go` dan
   `internal/filters/registry.go`. Ikutin spec di ROADMAP Fase 2.

5. **Stop setelah Fase 2 selesai.** Jalanin test, commit, report ke
   gue pake format di RULES.md bagian "Post-phase report format".
   Tunggu approval sebelum lanjut Fase 3.

## Prinsip non-negotiable

- 5 prinsip zero-risk di CONTEXT.md — ga boleh dilanggar demi token saving
- Zero external dependencies — stdlib only
- Incremental, per-fase — jangan multi-fase
- Test before commit — `go test && go vet` wajib hijau

## Yang gue ga mau lu lakuin

- Jangan "improve" runner.go atau stash.go tanpa alasan kuat
- Jangan add dependency tanpa propose dulu
- Jangan skip fase
- Jangan implement feature yang ga di ROADMAP v0.1.0
- Jangan kerja >30 menit stuck tanpa report

## Yang gue mau lu lakuin

- Tanya kalau ragu
- Propose trade-off eksplisit kalau ada design decision
- Minta feedback di akhir tiap fase
- Kasih tau kalau lu spot potential issue di depan

Udah siap? Start dengan baca 3 dokumen itu, lalu report:
1. Summary singkat apa yang lu pahami dari goal project
2. Konfirmasi zip udah di-extract dan rename sukses
3. Hasil `go build && go test` (pasang output apa adanya)

Baru setelah 3 itu clear, mulai Fase 2.
