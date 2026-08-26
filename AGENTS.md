# AGENTS.md

Go CLI (`cobra`) for bulk file processing: extract archives, normalize names, compute checksums.

## Commands

- Build: `go build ./...` or `make build`
- Run: `go run . <subcommand>`
- Test: `go test ./...` (or `make test`, `make cover`)
- Lint: `golangci-lint run` (`make lint`); CI runs it via `.github/workflows/ci.yml`
- Verify before finishing: `go build ./... && go vet ./... && go test ./...`

## Architecture

- Module path is the non-URL name `asset` (see `go.mod`); internal imports use `asset/cmd`.
- Entry point `main.go` just calls `cmd.Execute()`. All logic lives in `cmd/`.
- Subcommands (each in its own file): `extract`, `normalize`, `checksum`, `pipeline`.
- The real logic is in exported functions so `pipeline` can reuse them:
  `cmd/extract.go` `RunExtractionLogic`, `cmd/normalize.go` `RunNormalizationLogic`,
  `cmd/checksum.go` `RunChecksumLogic`. New subcommands should follow this split (thin cobra `RunE` wrapper + exported `Run...Logic` func).
- `pipeline` chains Extract -> Normalize -> Checksum -> Move, staged through a temp dir and moved to `--dest` via `moveFileRobust` (rename, with copy+remove fallback on cross-device link errors).
- Shared CLI state lives in `cmd/root.go`: `verbose`, `workers` (persistent flags) and helpers `debugf`/`warnf`/`numWorkers`.

## Gotchas

- User-facing strings are in Spanish; match that style.
- `normalize` renames deepest paths first (sorts by path-separator count, descending) so child renames aren't broken by parent renames. Preserve this ordering if you touch it.
- Extract supports `.zip`, `.tar.gz`/`.tgz`, `.rar`, `.7z` (no bare `.gz`); `detectArchiveType` decides by suffix. It preserves the source subdirectory structure in `dest`, and after extraction "flattens" archives that contain a single subdirectory (`flattenSingleDirectory` in `cmd/extract.go`).
- `checksum` writes a `checksums.txt` (SHA-256 + relative path) into the processed directory; the checksums themselves are not printed to stdout.
- Version is injected via `-ldflags "-X asset/cmd.version=<v>"` (default `dev`); `goreleaser` handles this in `.goreleaser.yml`.
- `RunNormalizationLogic(dir, dryRun)` and `RunChecksumLogic(dir, outputFile)` take extra args; keep the pipeline calls in sync if you change signatures.


