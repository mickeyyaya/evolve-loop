# Build Explanation — Cycle 1969

## Build Binding
- Cycle: 1969
- Base SHA: 79e2bb51e5b6db69ae6c13885484310c3270e6ad

## Summary
A new package, `internal/handoff/exit`, decides the exit of a handoff round: `pass`, `retry` with the ids of the unmet required asks, or `cap`.

## Rationale
ADR-0141 requires a deterministic exit with a stop on no gain. A pure function keeps the judgment out of the prompt. The package imports only `internal/handoff`, so the types stay a leaf.

## Changed Areas
- `go/internal/handoff/exit/exit.go` — adds `Decide`, `Decision`, `Outcome`, `Pass`, `Retry`, `Cap`, `Policy`, `Result` and `Round`.
- `go/internal/handoff/exit/apicover_named_test.go` — names every exported symbol in a real assertion.
- `go/.apicover-enforce` — enrolls `./internal/handoff/exit` in the apicover gate.
- `go/.docgo-allow-missing` — lists `./internal/handoff/exit`, as the sibling evidence package does, until the package has its own doc.
- `docs/architecture/packages/internal-handoff.md` — documents the exits.

## Design Decisions
`pass` is checked first, so a full pass wins over a spent cap. `cap` is checked before the no-gain test. The no-gain test compares the met asks of the request with the union of all earlier rounds. An ask is met only when all its results are `met` with resolved evidence.

## Verification
The table test in `exit_test.go` and the ACS predicates `TestC1969_001` to `009` cover each exit and the negative cases.

## Compatibility
The package is new; no existing caller changes.

## Limitations
No production code calls `Decide` yet. The loop wiring is a later step of plan H4.
