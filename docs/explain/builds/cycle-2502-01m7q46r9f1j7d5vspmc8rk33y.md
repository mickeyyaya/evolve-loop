# Build Explanation — Cycle 2502

## Build Binding
- Cycle: 2502
- Base SHA: 883c2ed5a6a661652c43b5d7e858d6dbd5d688bd

## Summary
A new leaf package, `internal/evidencex`, extracts a bounded, redacted summary from a log and records the digest of the whole raw source.

## Rationale
The incident knowledge base needs a fixed pipeline with a pointer mode and an extract mode (I12). A pure function with no clock and no map order gives the same bytes for the same input.

## Changed Areas
- `go/internal/evidencex/evidencex.go` — adds `Extract`, `Options`, `Result`, `Range`, `Template` and the two modes.
- `go/internal/evidencex/apicover_named_test.go` — names every export in a real run.
- `go/.apicover-enforce` — enrols the new package in the repo-wide apicover gate.
- `docs/architecture/packages/internal-evidencex.md` — records the design and invariants of the package.

## Design Decisions
Error windows outrank the tail, and the tail outranks templates, so a tight budget drops repeats first. `internal/redact` is reused for secrets, so the repository keeps one pattern list.

## Verification
The ACS predicates TestC2502_001 to TestC2502_010 and the package tests pass with `-count=1`.

## Compatibility
The package is new and has no existing caller, so no behavior changes.

## Limitations
No production code calls `Extract` yet. The incident bundle caller is a later item.
