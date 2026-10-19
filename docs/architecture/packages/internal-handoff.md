# internal/handoff

> Spec of record: [phase-handoff-contract](../phase-handoff-contract.md). Architecture decision: [ADR-0141](../adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md). Delivery plan: [phase-handoff-contract-2026-10](../../plans/phase-handoff-contract-2026-10.md).

## Purpose

`internal/handoff` defines the data types for phase handoffs. It defines the request types and the ledger schema. It parses and validates handoff ledgers.

## Design

- **Leaf package.** The package has no internal imports. It depends on standard library packages only.
- **Request and ledger types.** A `Request` contains asks. A `Ledger` contains answers for each ask of a request.
- **Deterministic validation.** `Validate` checks the ledger against the request. It confirms pair names, request digests, and entry counts.
- **Unified errors.** `Validate` reports all detected violations at once with `errors.Join`.
- **Evidence resolvers.** `internal/handoff/evidence` decides in code whether an `Evidence` pointer resolves. `Resolve` returns `ErrUnresolved` when it does not, and the entry is then unmet.
- **Digest binding.** `Request.Digest` computes a SHA-256 digest of the request JSON payload.

## Evidence kinds

| Kind | Ref form | Resolves when |
|---|---|---|
| `path` | `repo/path:line` | The file exists under the root and the line is within its line count. |
| `test` | `Name` or `file_test.go:Name` | A `_test.go` file declares the `Test`, `Fuzz` or `Benchmark` function. |
| `predicate` | `Name` or `go/acs/cycle<N>/file_test.go:Name` | The package of the cycle declares the function. |
| `command` | the command line | The `Source` records the command result. |

An unknown kind never resolves.

## Invariants

- **Schema version is one.** `Parse` rejects every ledger with a schema version other than 1.
- **Unknown asks fail.** An entry for an unknown ask causes a validation failure.
- **Stale digests fail.** A ledger with a mismatched request digest causes a validation failure.
- **Notes for not-applicable entries.** Each `not_applicable` entry must contain a non-empty note.
- **Zero comments.** The package code carries no comments.

## Findings

- None.
