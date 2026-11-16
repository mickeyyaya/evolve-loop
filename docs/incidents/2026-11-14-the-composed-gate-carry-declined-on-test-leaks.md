# 2026-11-14: the composed-tree gate of the identity carry was red on every tree because tests leaked files

Related documents: [ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md) (the identity carry) and [ADR-0166](../architecture/adr/0166-composed-gates-test-only-the-affected-packages.md) (composed gates test only the affected packages).

## Summary

- **Impact:** 11 of 14 identity carries in waves 171-172 ended with the signal `ORCHESTRATOR_COMPOSED_GATE_DECLINED`. Each decline cost 11-17 minutes per run at 4 lanes and 79 minutes at 8 lanes. The lane holds the ship window during this time.
- **Root cause:** the composed `test` gate runs `make -C go test`. The treewalk guard of that target fails the run on any file that the tests leave in `go/`. Several tests leave files, so the gate was red on every tree.
- **Evidence of the cause:** on a clean `origin/main` (`47a370296`) with the environment of the gate, `make test` failed with rc=2 in 9 minutes. It listed 11 leaked paths and no test FAIL.
- **Why CI did not see it:** CI runs `make test-integration`, which has no treewalk guard.
- **Fix:** a test-only change. Each leaking test names its own root. The production refactor is a separate inbox item.

## Terms used throughout

- **The identity carry:** `Orchestrator.identityCarryForward` (`go/internal/core/identity_carry_forward.go:18`). When a rebase is byte-identical, the audited verdict carries, and the cycle ships without a second audit.
- **The composed-tree gate:** the set of gates that run on the composed tree before a carry. A carry needs every gate to pass.
- **The treewalk guard:** the `evolve treewalk snapshot` and `evolve treewalk check` steps of the `test` target (`go/Makefile:107-112`).
- **A leak:** a file or an empty directory that a test leaves in `go/` after the run.

## Timeline

The fact sheet gives dates for waves, not times for most events. The one exact pair is below.

| Time | Event | Evidence |
|---|---|---|
| waves 171-172 | 11 of 14 identity carries end with `ORCHESTRATOR_COMPOSED_GATE_DECLINED`. 3 carry. | signal `ORCHESTRATOR_COMPOSED_GATE_DECLINED` (`go/internal/core/composition_carryforward.go:17`) |
| waves 171-172 | The 3 carries are cycle 2358 and cycle 2375 (twice). Each had an identical rebase and all gates passed. | composition verdict of each run |
| 08:53:23 to 10:12:34 | Cycle 2374 at 8 lanes: the ship error, then the decline, 79 minutes later. | ship error and decline of cycle 2374 |
| not dated | A run of `make test` on clean `origin/main` (`47a370296`) fails with rc=2 in 9 minutes. It lists 11 leaked paths. | the bisect run |
| not dated | Cycle 2375 lands ADR-0166, which narrows the cost. | ADR-0166 |

## Root cause

**F1.1** The identity carry runs the composed gates before it carries. The `test` gate runs `make -C go test`.
*Implication:* the carry depends on the result of that target.

**F1.2** The `test` target takes a snapshot of `go/`, runs the tests, and then runs `evolve treewalk check` against the snapshot (`go/Makefile:107-112`). The check fails the run on any file or empty directory that the tests left.
*Implication:* a test that leaves one file turns the whole target red, even when no test FAILs.

**F1.3** On a clean `origin/main` (`47a370296`) with the environment of the gate, `make test` failed with rc=2 in 9 minutes. It listed 11 leaked paths and no test FAIL.
*Implication:* the gate was red on every tree, not on the tree of one lane.

**F1.4** A bisect found these leakers:

- `TestHostChain_AnUnroutedPinnedItemTheHostClaimsPassesTheGate` (`go/internal/phases/triage/host_chain_test.go`) and `TestNewHostEffects_AProtectedFilesItemIsRoutedAndAClaimableItemIsClaimed` (`go/cmd/evolve/cmd_cycle_host_effects_test.go`). They build `inboxmover.Options` with only `InboxDir`. `resolveOpts` then derives a relative `.evolve/ledger.jsonl`.
- `TestRetroStateWriteFailuresAreNonFatal` (`go/internal/core/coverfull_records_test.go`). It passes an empty `CycleRequest.ProjectRoot`.

A second pass found two more: `TestRecordAndBranch_ChecksTheCycleSpendAtEachBoundary` (`go/internal/core/cycle_spend_limit_test.go`) and `TestLaneResume_ABindingWhoseAncestorLeftNoLedgerIsReleasedAndTheGateDoesNotBlock` (`go/cmd/evolve/cmd_cycle_lane_resume_binding_test.go`).
*Implication:* a missing root makes a path relative to the working directory, and that directory is `go/` or a package directory.

**F1.5** CI runs `make test-integration` (`go/Makefile:127`). That target has no treewalk step.
*Implication:* CI never saw the leaks. Only the composed gate ran the guard.

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Keep the carry. | The carry succeeded 3 times. A success writes no signal, so a count of declines alone was misleading. The carry ships while the lane holds the ship window. A re-audit does not (the rebase chase). | Retire the carry (option R). It removes a path that works when the gate is green. |
| D2 | Fix the leakers in the tests only (L-min). Each test names its root. | F1.3, F1.4 | A production refactor now. It was parked (see below). |
| D3 | Keep the cost fix separate. ADR-0166 makes the composed gates test only the affected packages. | F1.1 | None. Cycle 2375 landed it. |

The production refactor is: every inbox mover gets its root from one source, and CI runs the treewalk guard. Two designs went through 5 review rounds: root inference, and an explicit root with a guard. Neither was accepted. It is parked and filed as its own item.

## What changed

The fact sheet names no pull request for the test fixes, so none is cited here.

| Change | Regression test |
|---|---|
| ADR-0166: composed gates test only the affected packages (cycle 2375) | not named in the fact sheet |
| The five leaking tests of F1.4 name their root | the `make -C go test` run on a clean tree: the treewalk guard passes |

## What we learned

- A guard that runs in one gate and not in CI will fail in that gate alone. The production refactor, which also makes CI run the treewalk guard, is the open item.
- A count of declines is not a measure when a success writes no signal. Count both outcomes before you retire a path.
- A test that gives a mover only a partial set of options lets the mover derive a relative path. Each test must name its root.
