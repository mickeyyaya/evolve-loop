# Build Explanation — Cycle 2390

## Build Binding
- Cycle: 2390
- Base SHA: 6ed2bd5bb11546beabf97250eb67f6556bfdf17f

## Summary
The `tdd-build` handoff request and the handoff ledger reader now read files through one bounded plain-file reader in `go/internal/plainfile`.
The reader refuses a FIFO, a symlink, a hard-linked file, a swapped file, and a file above 4 MiB.

## Rationale
`request.TDDBuild` previously called `parser.ParseFile` directly on `_test.go` paths, which can hang on a FIFO or leak bytes from a symlink target.
`core.ReadHandoffLedger` previously called `io.ReadAll` without a byte cap.
A new leaf package `go/internal/plainfile` lets both `go/internal/handoff/request` and `go/internal/core` share one reader without an import cycle or edits to `go/internal/inputindex`.

## Changed Areas
- `go/.apicover-enforce` — enrolls `./internal/plainfile` in the repository API coverage gate.
- `go/internal/core/handoff_reviewer.go` — delegates `ReadHandoffLedger` to `plainfile.Read` with a 4 MiB cap (`maxHandoffLedgerBytes`).
- `go/internal/handoff/request/request.go` — reads each `_test.go` file through `plainfile.Read` with a 4 MiB cap (`maxTestFileBytes`) and passes the bytes to `parser.ParseFile`, skipping `plainfile.ErrNotPlain` entries.
- `go/internal/plainfile/doc.go` — adds the package doc comment for `plainfile`.
- `go/internal/plainfile/plainfile.go` — implements `Open`, `Read`, and `ReadWith` with `os.Lstat`, `IsRegular`, `inputindex.HardLinked`, `O_NONBLOCK|O_NOFOLLOW`, `os.SameFile`, and `io.LimitReader`.

## Design Decisions
Both `request.TDDBuild` and `core.ReadHandoffLedger` call `plainfile.Read` rather than duplicating `Lstat` and `O_NONBLOCK` checks.
The reader passes in-memory bytes to `parser.ParseFile` instead of checking a path before `ParseFile` opens it again.
`plainfile` reuses `inputindex.HardLinked` rather than defining a second hard-link helper.

## Verification
All 11 cycle-2390 ACS predicates in `go/acs/cycle2390` pass (`TestC2390_001` through `TestC2390_011`), including 100% statement coverage and zero code comments.
Targeted unit tests pass in `go/internal/plainfile`, `go/internal/handoff/request`, and `go/internal/core`.

## Compatibility
`core.ReadHandoffLedger` keeps its signature and returns `plainfile.ErrNotPlain` and `plainfile.ErrTooLarge` for non-regular or oversized files.
`request.TDDBuild` keeps its signature and skips non-plain `_test.go` entries while still failing on syntax errors or missing predicates.

## Limitations
Both callers enforce a fixed 4 MiB cap (`4 << 20` bytes) rather than a configurable policy field.
