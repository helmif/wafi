# wafi — Rules for Claude Code

> Aturan yang harus dipatuhi Claude Code saat ngerjain project ini.

## Workflow rules

1. **Baca CONTEXT.md + ROADMAP.md setiap session start.** Ga boleh
   proceed tanpa refresh context.

2. **Kerjain satu fase per session.** Selesai fase → commit → stop →
   report ke user. Jangan lanjut fase berikutnya tanpa approval.

3. **Run test + vet sebelum commit:**
   ```bash
   go test ./... && go vet ./... && go build ./...
   ```
   Semua harus hijau. Kalau merah, fix dulu.

4. **Commit message format:**
   ```
   <phase>: <what>
   
   <why, if not obvious>
   
   Refs: ROADMAP.md Fase <N>
   ```
   Contoh: `filters: add git_status filter with 8 fixtures`

5. **Satu commit per logical unit.** Jangan bundling "fase 3 + beberapa
   fix dari fase 2" ke satu commit.

## Code rules

### Mandatory

1. **Zero external dependencies.** Package import cuma stdlib.
   Exception: kalau ada kebutuhan yang benar-benar butuh dep,
   propose ke user dulu, jangan langsung add.

2. **Safety invariants di header file.** Setiap file di `internal/`
   WAJIB punya comment block di atas yang list safety invariants.
   Contoh ada di runner.go dan stash.go.

3. **Error handling explicit.** Ga boleh `_ = someFunc()` tanpa komentar.
   Kalau sengaja ignore, kasih alasan di komentar.

4. **No panic di library code.** Package `internal/` ga boleh panic.
   Return error. `cmd/wafi/` boleh pake `log.Fatal` di main path.

5. **Path sanitization.** Semua path yang berasal dari user input atau
   env WAJIB lewat `filepath.Clean` dan validasi relatif.

### Forbidden patterns

1. **`sh -c` atau shell interpretation.** Pake `exec.Command(name, args...)`
   langsung.

2. **`os.Getenv` tanpa default.** Semua env read harus handle empty case.

3. **Logging secret.** Jangan print env vars, config file content, atau
   command args yang might contain secrets ke stderr/stdout.

4. **Net/http imports.** Tool ini 100% local. No network calls.

5. **`go get` untuk add dep.** Propose dulu.

6. **Refactor file yang udah ada tanpa alasan.** runner.go dan stash.go
   udah reviewed. Perubahan harus justified.

## Testing rules

1. **Setiap filter punya `_test.go`.** Minimum 3 test case:
   happy path, edge case, malformed input (pass-through).

2. **Testdata di `testdata/` per package.** Naming:
   `<filter>_<scenario>.txt`. Example: `git_status_clean.txt`.

3. **Golden files untuk expected output.**
   `testdata/<filter>_<scenario>.golden.txt`. Update via
   `go test -update` (bikin helper di testutil).

4. **No test yang panggil real external command** di unit test.
   Pake fixture. Integration test terpisah di `integration/`.

5. **Table-driven tests** untuk filter. Pattern:
   ```go
   tests := []struct{
       name   string
       input  string
       exit   int
       want   string
   }{...}
   ```

## Documentation rules

1. **GoDoc di public API.** Setiap exported type/func di
   `internal/` wajib punya doc comment (walaupun internal, buat
   self-documentation).

2. **README updates per fase.** Update section "Supported commands"
   tiap kali tambah filter.

3. **CHANGELOG.md.** Update di akhir tiap fase dengan format:
   ```
   ## [Unreleased]
   
   ### Added
   - git_status filter with 8 test fixtures
   ```

## Communication rules

1. **Kalau ragu, tanya.** Lebih baik pause dan konfirmasi daripada
   implement yang salah arah.

2. **Kalau hit blocker, report.** Jangan spend >30 menit stuck tanpa
   report ke user.

3. **Propose trade-off explicit.** Kalau ada pilihan design,
   sebutin 2-3 opsi + rekomendasi, jangan pilih unilateral.

4. **Post-phase report format:**
   ```
   ## Fase <N> selesai
   
   ### Apa yang ditambah
   - ...
   
   ### Apa yang di-test
   - ... (coverage %)
   
   ### Keputusan yang diambil
   - ... (kalau ada yang non-obvious)
   
   ### Catatan untuk fase berikutnya
   - ... (potential issue, dependency, dll)
   ```

## Security review checklist (sebelum release v0.1.0)

- [ ] `go vet ./...` clean
- [ ] `gosec ./...` clean (install via `go install github.com/securego/gosec/v2/cmd/gosec@latest`)
- [ ] Tidak ada `sh -c`, `os/exec.Command` dengan shell interpretation
- [ ] Semua file write pake mode 0600 atau more restrictive
- [ ] Semua dir create pake mode 0700 atau more restrictive
- [ ] Ga ada network call (grep `net/http`, `net.Dial`)
- [ ] Ga ada telemetry / analytics
- [ ] Env vars ga di-log
- [ ] Config file parsing handle malformed gracefully
- [ ] Panic recovery di filter dispatch
- [ ] Exit code preserved di semua path

Kalau ada item yang fail, jangan release.
