# Build Explanation — Cycle 2456

## Build Binding
- Cycle: 2456
- Base SHA: ed3bf7d91654154ae0d35b697bd017017f0cf911

## Summary
The integration train now settles landed and closed PRs at the start of each batch run. It spends their records, closes landed PRs, and closes batch PRs that hold nothing open.

## Rationale
A batch that stopped after it moved main left the records of the landed PRs in place. The settle ran only at the end of the batch that landed them. Running the settle at the next batch start repairs that state with no manual step.

## Changed Areas
- `go/internal/integration/settle.go` — adds `Train.Settle`, the record settle, the batch PR settle and the `Source-PR` reader.
- `go/internal/integration/train.go` — adds `State` and `OpenBatches` to the `PRHost` interface.
- `go/internal/integration/ghci.go` — adds the `gh` reads `GhHost.State` and `GhHost.OpenBatches`.
- `go/cmd/evolve/cmd_integration_batch.go` — calls the settle before the plan of a non-dry batch, prints its lines and drops a settled PR from the named operands.
- `go/cmd/evolve/cmd_integration_batch_test.go` — the shared `gh` fake answers `pr view` and an empty batch list, so older journeys reach the settle.
- `go/cmd/evolve/cmd_integration_wiring_test.go` — the unreadable-directory test supplies a `gh` fake, because the settle now reads the host.
- `go/cmd/evolve/cmd_integration_settle_test.go` — three journeys for the settle, each driven through the real batch entry point.
- `go/internal/integration/train_test.go` — the `fakeHost` implements the two new host reads.
- `go/internal/integration/settle_test.go` — names each new export in an executed assertion.
- `docs/architecture/packages/internal-integration.md` — records the settle design and its invariants.

## Design Decisions
The settle reuses `Train.complete`, so a landed PR keeps one path: inbox move, close, record spend. A batch PR closes only when every commit names a `Source-PR` and every named PR is gone. Unknown membership keeps the PR open.

## Verification
The three journey tests in `go/cmd/evolve/cmd_integration_settle_test.go` and the three ACS predicates of cycle 2456 pass. The integration package tests pass.

## Compatibility
No flag and no policy value is new. A dry run settles nothing. The close note of a landed PR is unchanged.

## Limitations
A PR merged on the host without a landing through the train has its record spent, and its inbox items do not move.
