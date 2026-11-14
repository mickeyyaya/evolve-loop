# Build Explanation — Cycle 2368

## Build Binding
- Cycle: 2368
- Base SHA: ecd6fc44ed2a712664e57bd97f7823add9efc3ff

## Summary
The tdd-build handoff request and the handoff ledger read now go through one plain-file reader. The reader refuses a symlink, a hard link, a FIFO and a swapped file, and it caps the size.

## Rationale
A FIFO named like a test file blocked the directory read. A symlink or a hard link read bytes from outside the package. An unbounded ledger read allocated the whole file. One reader holds each check, so the two call sites cannot drift apart.

## Changed Areas
- `go/internal/plainfile/plainfile.go` — the new reader. It uses an `O_NONBLOCK|O_NOFOLLOW` open, an fstat `SameFile` check, a size limit and `inputindex.HardLinked`.
- `go/internal/plainfile/doc.go` — the one package doc that the docgo gate requires.
- `go/internal/handoff/request/request.go` — `TDDBuild` reads each test file through `plainfile.Read` and parses the bytes. It skips a file that is not plain and fails on any other read error.
- `go/internal/core/handoff_reviewer.go` — `ReadHandoffLedger` calls `plainfile.Read` with a 4 MiB cap. The two ledger errors alias the reader errors.
- `go/internal/core/handoff_persona_test.go` — the swap and FIFO tests call the shared reader.
- `go/internal/core/handoff_ledger_refusals_test.go` — tests for the missing, symlinked and oversize ledger refusals.
- `go/internal/handoff/request/tdd_build_oversize_test.go` — a test that an oversize test file fails the request.
- `go/internal/plainfile/plainfile_test.go` — tests for each refusal of the reader.
- `go/internal/plainfile/apicover_named_test.go` — names each exported symbol of the package.
- `go/.apicover-enforce` — enrolls `./internal/plainfile` in the apicover gate.

## Design Decisions
The reader sits in its own package, so that `request` and `core` share it without a cycle. The hard-link test is `inputindex.HardLinked`; `inputindex` does not import `core` or `handoff`. The parser receives the bytes that were read, so it never reopens the path.

## Verification
`go test -tags acs ./acs/cycle2368` passes all ten predicates. `go test ./internal/plainfile ./internal/core ./internal/handoff/...` passes.

## Compatibility
Ordinary regular test files and ledgers below 4 MiB behave as before.

## Limitations
The 4 MiB caps are constants, not policy settings.
