# Build Explanation — Cycle 2537

## Build Binding
- Cycle: 2537
- Base SHA: f31324cca92aa50ea853c803fe43e130f457bd05

## Summary
The `internal/evidencex` package now provides the `EvidenceArchive` port and a local directory adapter (`NewLocalArchive`) keyed by `source_sha256` with size-cap enforcement.

## Rationale
Decision D17 needs a content-addressed archive port for `full` evidence mode. Raw logs stay outside git, and writes above the size cap fail before storage.

## Changed Areas
- `go/internal/evidencex/archive.go` — adds `EvidenceArchive`, `NewLocalArchive`, `ErrArchiveTooLarge`, `ErrArchiveNotFound`, key validation, atomic writes, and an unexported `archiveCodec` seam with a `compress/gzip` default.
- `go/internal/evidencex/archive_test.go` — adds unit tests for round-trip persistence, raw-byte size caps, duplicate writes, key validation, and corrupt payload handling.
- `go/internal/evidencex/archive_internal_test.go` — adds internal unit tests covering codec, temp-file, and atomic-rename error paths.
- `go/internal/evidencex/apicover_named_test.go` — names and executes every new exported symbol in `internal/evidencex`.
- `docs/architecture/packages/internal-evidencex.md` — documents the archive port, codec seam, safety rules, and `NewLocalArchive` apicover invariant.

## Design Decisions
Because `go/go.mod` and the module cache lack a zstd package, `archive.go` isolates compression behind an unexported `archiveCodec` interface. The local adapter uses standard-library `compress/gzip` as the default codec. `Put` checks `maxBytes` against raw source bytes before any disk write, and validates 64-character lowercase hex keys. It skips existing objects on duplicate writes, and commits new objects through a temp file and rename.

## Verification
All unit tests in `go/internal/evidencex/`, ACS predicates `TestC2537_001` through `TestC2537_008` in `go/acs/cycle2537`, `go/acs/cycle2502`, and `apicover -enforce ./internal/evidencex` pass with `-count=1`.

## Compatibility
Existing `Extract`, `ModeExtract`, and `ModePointer` APIs and callers remain unchanged.

## Limitations
The local adapter uses `compress/gzip` instead of zstd until a zstd dependency is added to `go/go.mod`. No production caller in `cmd/evolve` invokes `NewLocalArchive` yet because `incidentcapture` and `full` mode wiring are deferred to later items.
