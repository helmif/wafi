# Contributing

Thanks for considering a contribution. wafi stays a single static binary
with zero runtime dependencies, so patches that add third-party modules
will not be accepted. Standard library only.

## Adding a new filter

1. Create `internal/filters/<name>.go` with a type that implements the
   `Filter` interface:

   ```go
   type Filter interface {
       Name() string
       Match(cmd string, args []string) bool
       Apply(stdout []byte, ctx ApplyContext) []byte
   }
   ```

   `Match` must be cheap and side-effect free. `Apply` must be
   deterministic: regex and string operations only, no I/O, no network.
   When the input falls outside the grammar the filter understands, return
   the input unchanged — "when in doubt, pass through" is a non-negotiable
   rule.

2. Register the filter in `internal/filters/registry.go` inside `Default()`.
   More specific filters register before general ones; first match wins.

3. Add real-world fixtures under `internal/filters/testdata/`. Real output
   captured from an actual command is preferred over synthetic samples.
   Each filter needs **at least 3 test cases**, one of which exercises the
   passthrough path (unknown or malformed input).

4. Regenerate golden files with:

   ```bash
   go test ./internal/filters/ -update
   ```

   Inspect the diffs before committing — `-update` writes whatever the
   filter currently produces.

5. Keep coverage above 80%. `go test -cover ./internal/filters/` should
   stay comfortably above that threshold.

6. Update `cmd/wafi/main.go` `knownFilteredCmds` if your filter handles a
   binary that isn't already there. This is what the `PreToolUse` hook
   uses to decide which Bash commands to prefix with `wafi run`.

7. Document the filter in `CHANGELOG.md` and add a row to the supported-
   commands table in `README.md`.

## PR checklist

Before opening a PR, please confirm:

- [ ] `go test ./...` passes
- [ ] `go test -tags integration ./integration/` passes
- [ ] `go vet ./...` is clean
- [ ] `gosec ./...` has no HIGH or MEDIUM findings (or they are annotated
      with `#nosec` and a justification)
- [ ] New filter has at least 3 test cases, one of which is a passthrough
      on unknown or malformed input
- [ ] `CHANGELOG.md` updated
- [ ] `README.md` supported-commands table updated if a new filter was added

## Running the full suite

```bash
go build ./...
go test ./...
go test -tags integration ./integration/
go vet ./...
gosec ./...
```

Cross-compile sanity check:

```bash
GOOS=linux   GOARCH=amd64 go build -o /tmp/wafi-linux-amd64   ./cmd/wafi
GOOS=linux   GOARCH=arm64 go build -o /tmp/wafi-linux-arm64   ./cmd/wafi
GOOS=darwin  GOARCH=amd64 go build -o /tmp/wafi-darwin-amd64  ./cmd/wafi
GOOS=darwin  GOARCH=arm64 go build -o /tmp/wafi-darwin-arm64  ./cmd/wafi
GOOS=windows GOARCH=amd64 go build -o /tmp/wafi-windows.exe   ./cmd/wafi
```
