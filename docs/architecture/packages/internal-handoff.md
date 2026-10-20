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
- **Check record.** `Check` is the result of the check of one ledger. It holds the stage, the outcome, the unmet and blocked asks, the proof paths, the failed evidence and the errors. `core.HandoffReviewer` writes it to `Ledger.Check` (key `check`, omitted when empty). `IsCheckOnly` is true for a file with a check and no ledger fields, and each reader treats that file as an absent ledger.

- **Exit decision.** `internal/handoff/exit` decides the exit of a round. `Decide` is pure: it takes the request, the check results, the earlier rounds and a `Policy`, and returns a `Decision`.
- **Blocked exit.** A `blocked` result carries a `Claim`. `Decide` hands the claims to the injected `Policy.Prover`. The exit is `blocked` only when the prover returns `Blocked` and every unmet required ask is named by a proven claim. The console wires the prover to `core.NewBlockerProver`, so `exit` imports neither `core` nor `guards` and holds no protected-path matcher. An unproven claim stays an unmet ask.

- **Request builders.** `internal/handoff/request` builds the `Request` of each pair from one source. It does not import `core`, and a test enforces this.
  - `ScoutTriage` makes one ask per selected slug. It trims, drops blanks and repeated slugs.
  - `TriageTDD` reads the committed set and the inbox record of each committed id. It makes one ask per acceptance entry, with the inbox source and the verbatim text. It returns `ErrNoCommittedSet` or `ErrUnresolvedItem` when it cannot build the whole request.
  - `TriageTDDFromItems` is the pure part of `TriageTDD`: it takes loaded inbox items.
  - `TDDBuild` parses the `_test.go` files of an ACS cycle package and makes one ask per predicate. It returns `ErrNoPredicates` for a package without one.

## Exits

| Outcome | When | `Unmet` |
|---|---|---|
| `pass` | Each required ask is met with resolved evidence. An optional unmet ask does not block. | empty |
| `cap` | A required ask is unmet and the redispatch cap is spent, or the round met no ask that no earlier round met. | empty |
| `retry` | A required ask is unmet, the cap has room and the round gained. | the ids of the unmet required asks, in request order |

`pass` wins over `cap`. An ask with no result, unresolved evidence, a status other than `met` or conflicting results is unmet. The first round has no earlier round, so it never stops on no gain. A `MaxRedispatch` of zero or less caps the first failed round.

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
