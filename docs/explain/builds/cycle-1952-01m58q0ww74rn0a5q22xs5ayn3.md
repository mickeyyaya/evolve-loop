# Build Explanation — Cycle 1952

## Build Binding
- Cycle: 1952
- Base SHA: c24feca09f4796399f364883c2b354c23ee64d46

## Summary
Implements phase handoff domain types, ledger schema, parsing and validation in new package `go/internal/handoff`. Enrolls the package in 100% strict coverage and API cover enforcement, and documents the architecture contract.

## Rationale
Phase pairs need deterministic validation of handoff ledgers against their requests before proceeding. Creating `go/internal/handoff` as a standalone leaf package provides the core domain types and validation logic without dependencies on orchestrator or runner packages.

## Changed Areas
- `go/internal/handoff/types.go` — defines domain types `Request`, `Ask`, `Ledger`, `Entry`, `Evidence`, `Author`, status and exit constants, and SHA-256 digest computation.
- `go/internal/handoff/validate.go` — implements `Parse` and `Validate` functions with sentinel errors for schema version, pairs, digests, asks, entries, and notes.
- `go/internal/handoff/types_test.go` — unit tests for types, JSON keys matching spec, and request digest behavior.
- `go/internal/handoff/validate_test.go` — unit tests for parsing, round-trip fidelity, and comprehensive refusal validation table.
- `go/acs/cycle1952/predicates_test.go` — acceptance predicates for cycle 1952 verifying validation refusals, coverage, and documentation links.
- `.evolve/evals/handoff-h1-types-ledger.md` — eval graders defining regression test expectations.
- `docs/architecture/packages/internal-handoff.md` — package documentation in ASD-STE100 format linking the phase handoff contract specification and delivery plan.
- `go/.cover-strict` — enlists `./internal/handoff` at 100% statement coverage.
- `go/.apicover-enforce` — enlists `./internal/handoff` for complete exported symbol coverage.
- `go/.docgo-allow-missing` — lists `./internal/handoff` to satisfy docgo invariant while respecting comment-free code rule.

## Design Decisions
The package is designed as a leaf package with standard library dependencies only. `Validate` reports all detected violations at once using `errors.Join`, allowing caller phases to fix all violations in one round. In accordance with the repository comment-free code convention, `go/internal/handoff` contains zero source comments.

## Verification
Unit tests in `go/internal/handoff` pass with 100.0% statement coverage. API cover enforcement passes with 27/27 exported symbols verified. All 7 ACS predicates in `go/acs/cycle1952` pass.

## Compatibility
`go/internal/handoff` is a newly introduced leaf package; existing packages and APIs remain unaffected.

## Limitations
This change delivers component H1 (types and ledger validation). Request builders (H2), evidence resolvers (H3), exit decisions (H4), and core wiring (H8) are delivered in subsequent cycles.
