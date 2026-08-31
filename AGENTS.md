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
- Subcommands (each in its own file): `extract`, `normalize`, `checksum`, `process`, `ingest`, `scan`, `db`.
- The real logic lives in services under `internal/` (`extract.ExtractorService`,
  `normalize.NormalizerService`, `checksum.ChecksumService`, `asset.ScannerService`,
  `database`). Subcommands are thin cobra `RunE` wrappers that delegate to those services.
- `process` chains Extract -> Normalize -> Checksum -> Move, staged through a temp dir and
  moved to `--dest` via `moveFileRobust` (rename, with copy+remove fallback on cross-device link errors).
- `ingest` runs `process` and then indexes the result into SQLite via `asset.ScannerService`.
- SQLite driver is `modernc.org/sqlite` (pure Go, no cgo): register with `_ "modernc.org/sqlite"` and
  `sql.Open("sqlite", dsn)`. The DSN uses `_pragma=...` query params (not mattn's `_fk=`/`_journal_mode=`).
  Full-text search is FTS5 (`assets_fts`).
- Shared CLI state lives in `cmd/root.go`: `verbose`, `workers`, `syncWrites`, `dbPath`
  (persistent flags) and helpers `debugf`/`warnf`/`numWorkers`.

## Gotchas

- User-facing strings are in Spanish; match that style.
- `normalize` renames deepest paths first (sorts by path-separator count, descending) so child renames aren't broken by parent renames. Preserve this ordering if you touch it.
- Extract supports `.zip`, `.tar.gz`/`.tgz`, `.rar`, `.7z` (no bare `.gz`); `detectArchiveType` decides by suffix. It preserves the source subdirectory structure in `dest`, and after extraction "flattens" archives that contain a single subdirectory (`flattenSingleDirectory` in `cmd/extract.go`).
- `checksum` writes a `checksums.txt` (SHA-256 + relative path) into the processed directory; the checksums themselves are not printed to stdout.
- Version is injected via `-ldflags "-X asset/cmd.version=<v>"` (default `dev`); `goreleaser` handles this in `.goreleaser.yml`.
- `ExtractorService.ExtractAll(ctx, ExtractOptions)` takes a `context.Context` and an `ExtractOptions` struct (Src, Dest, Workers, Sync, MinFree, RemoveSource, ErrorDir, Password); `NormalizerService.NormalizeAll(dir, dryRun)` and `ChecksumService.ChecksumAll(dir, outputFile, workers)` keep their signatures. Platform-specific helpers (`checkFreeSpace`, `openFileNoFollow`, `isRotational`) live in build-tagged files (`sys_unix.go`, `sys_windows.go`, `disk_linux.go`, `disk_other.go`); keep those in sync per-OS if you change them.


