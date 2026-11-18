# Build Explanation — Cycle 2515

## Build Binding
- Cycle: 2515
- Base SHA: 650f180dce5a7259de054e473f9cf6a876e558b3

## Summary
The seat walker now has a candidate filter, and the package `handoff` has an independence filter for the future judge. The judge itself is not in this tree. `bridgechain.Walking` takes a candidate filter, returns the seat that served the request, and `handoff.Independents` builds the filter from the author of a ledger. A test pins `policy.TierNames` equal to `modelcatalog.CanonicalTiers`.

## Rationale
The judge of branch dev/cl-c1 walked its own seats with `ChainAttempt`. That path skipped the exit-85 bench and the fresh session. One walker with a filter keeps both and gives the judge no second walk to maintain.

## Changed Areas
- `go/internal/bridgechain/bridgechain.go` — adds `CandidateSpec`, `Seat`, `WithCandidateFilter` and `LaunchSeat`. `Launch` calls `LaunchSeat`. A `ChainAttempt` request meets the same filter.
- `go/internal/llmroute/llmroute.go` — adds the function `SameFamily`, which `handoff.Independents` calls, and the field `Plan.Admit`. `Plan.Permits` calls it with the canonical tier, after the tier name is normalized and before the ceiling is read.
- `go/internal/handoff/independents.go` — adds `Independents`. It admits another family, or the same family at a stronger tier, and refuses all seats for an unattributed author.
- `docs/architecture/phase-handoff-contract.md` — adds the section H10 and names the guard tests in the guards table.
- `go/internal/policy/tier_vocab_test.go` — pins `policy.TierNames` and `policy.TierRank` to `modelcatalog.CanonicalTiers`.
- `go/internal/llmroute/tier_ceiling_test.go` — a test for `Plan.Admit` that proves it narrows and never widens the ceiling.

## Design Decisions
The filter sits in the plan, as `Plan.Admit`, not in the callback of the walk. `llmroute.DispatchTiered` then skips a refused seat without a launch, and the bench and the step-down logic stay unchanged. A copy of the ceiling map by family loses the difference between two drivers of one family. A second walker was rejected, because it caused the incident.

`Independents` fails closed. An empty or unknown author tier refuses a same-family seat. An empty or `unattributed` author CLI refuses every seat.

## Verification
The new tests in `go/internal/bridgechain`, `go/internal/handoff`, `go/internal/policy` and `go/internal/llmroute` pass. The cycle 2515 ACS predicates pass. Predicate 008 proves that the new tests fail on the base production files.

## Compatibility
A walker with no filter behaves as before. `Launch` keeps its signature. `go/internal/policy/core.go` is a protected path and stays unchanged. The drift test replaces the planned single source.

## Limitations
The judge itself is not in this tree. `WithCandidateFilter` and `Independents` have no production caller yet, only tests. The wiring of the judge is the next unit.
