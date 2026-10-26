# Retry-hook registry & spine fail-open telemetry

_Cycle-1166. Covers three inbox items that share one disease: a concept spelled
in more than one place, so the copies drift._

## 1. `retryOpts` — one registry of phase-retry recovery hooks

**Problem.** Two retry loops existed: the sequential dispatch loop
(`go/internal/core/cyclerun_dispatch.go`) and the evaluate-batch loop
(`go/internal/core/evaluate_batch.go`). They agreed only by hand, so every hook
the sequential loop grew had to be re-remembered on the batch side — and twice it
was not. `optionalInfraSkip` and `postShipObserverSkip` shipped sequential-only,
so an optional evaluate phase that exhausted infra retries aborted the *whole
batch* instead of degrading to WARN. A patch of the two misses does not fix the
class: the next hook diverges the same way.

**Design** (`go/internal/core/retry_opts.go`). `retryOpts` is a Strategy value
enumerating every recovery hook a retry loop may run:

| hook | meaning | main | resume | batch |
|---|---|---|---|---|
| `backfill` | reconstruct the artifact from `stdout.clean.txt` after an `ErrArtifactTimeout` exhaustion | ✅ | ✖ | ✖ (writes the workspace; the batch runs concurrently) |
| `optionalInfraSkip` | optional, off-floor phase with an infra-shaped exhaustion degrades to WARN | ✅ | ✅ | ✅ |
| `postShipObserverSkip` | best-effort post-ship Control observer's failure degrades to WARN | ✅ | ✅ | ✅ |
| `shipRecovery` | route a structured `ShipError` to the advisor's recovery chain | ✅ | ✖ (the loop calls `recoverResumedShip` after the dispatch) | ✖ (rewrites the loop cursor; an evaluate batch never contains ship) |

A `nil` field means "this path does not run that hook" — divergence declared and
inspectable, rather than implicit and discoverable only by diffing two loops.
`mainDispatchRetryOpts()` is the REFERENCE set. It wires every hook.
`evaluateBatchRetryOpts()` is the batch subset.

`resumeDispatchRetryOpts()` starts from the reference set and clears only
`backfill` and `shipRecovery`. A hook added to the reference set therefore
reaches the resume loop without a second edit.
The resume loop reads the ship latch from its checkpoint (`CycleState.Shipped`).
A best-effort observer that fails after Ship on a resumed cycle therefore
degrades to WARN, as it does on a fresh cycle.

The resume loop does not need the `shipRecovery` hook.
It calls `recoverResumedShip` (`go/internal/core/resume_ship_recovery.go`)
when a dispatch returns a ship error.
The call runs the recovery ladder of the fresh loop.
The loop then reroutes its own cursor to the phase that the ladder picks.
The allowed phases are Build, Audit and Ship.
A ladder route to any other phase ends the cycle with the ship error.

When the ladder declines, the loop ends the cycle with the error that `recoverResumedShip` returns.
For a stale base, that error is `GIT_FLEET_REBASE_NEEDED`, and `original_code` keeps the landing code.

A resumed ship that finds main past the checkpoint base returns
`GIT_FF_MERGE_DIVERGED`, because `evolve loop --resume` does not run in the fleet.
`staleBaseAsRebase` changes that code to `GIT_FLEET_REBASE_NEEDED`
when the base is behind `main`.
The ladder then rebases the lane and verifies it again before the next ship.

`retryPhaseRunner` is the shared retry core.
`dispatchRunnerWithRetry` is a one-line delegation to it.
The batch path therefore cannot keep a second hand-maintained loop.

The sequential loop keeps its own control flow. It owns aborts, ledger writes
and the cursor. It consults its hooks *through the registry*, so both paths
read from one enumeration.

**Guard.** `retry_opts_parity_test.go` reflects over `retryOpts`.
It asserts that the field set equals the canonical hook list.
A hook added to the dispatch loop without a field here fails the table.
The parity gap is structural and does not depend on vigilance.

`TestResumeDispatchRetryOpts_MatchesMainDispatchExceptDocumentedDivergence`
asserts that the resume set equals the reference set minus the two declared
omissions. A hook wired on one path only fails the test.

## 2. `IsInfraTeardownError` — the union predicate spelled once

`(errors.Is(err, ErrArtifactTimeout) || errors.Is(err, ErrTransientBridgeFailure))`
had a single-source home (`errors.go:IsInfraTeardownError`) but was still
hand-spelled at three call sites, one as the De Morgan negation. All three now
call the helper: `orchestrator.go:optionalInfraSkip`, the sequential retry arm,
and the batch arm (the batch arm calls it through the shared core).
`isTransientBridgeError` stays a **transient-ONLY component**. Never alias it to
the union. `TestIsTransientBridgeError_StaysTransientOnly` is the anti-widen
guard. Timeout-only sites (`failure_hook.go`, `failure_learning.go`, the
backfill trigger) are *different predicates*, and the change left them alone on
purpose.

`TestInfraTeardownUnion_SpelledExactlyOnce` parses every non-test file in
`internal/core` and counts boolean expressions joining the two sentinels.
Exactly one may exist. Because it is a uniqueness assertion, it cannot be
satisfied by adding text — only by removing duplicate spellings.

## 3. Spine fail-open telemetry

**Problem.** A width-3 batch on 2026-07-13 emitted 76 `spine not satisfied …
proceeding fail-open` WARNs to stderr and nowhere else: an epidemic with no
dashboard. This change is measurement-first — make it visible, *then* decide
if the fix is artifact-retry, driver-bounce recovery, or an enforce flip.

**Design.** The design follows the `VerdictsNotAdopted` precedent exactly (one
record, one projection — the field was named `SkippedPhases` until the record
split of `dossier-retro-skipped-mislabel`). It does not invent a new shape:

```
StateMachine.UnsatisfiedSpineAnchor   → names the FIRST unsatisfied predecessor
cycleRun.recordSpineFailOpen          → cyclestate.CycleResult.SpineFailOpens
writeCycleDossier                     → dossier.BuildOpts.SpineFailOpens
                                      → Dossier.spine_fail_opens (omitempty)
dossier.RollupSpineFailOpens          → batch Total / ByPhase / OverThresholdCycles
```

- `SpineSatisfiedUpTo` now **delegates** to `UnsatisfiedSpineAnchor`, so the gate
  and its reporter cannot disagree. A reporter that inflated the counter is
  worse than no telemetry.
- The record carries `(phase, missing_artifact, reason)`. Phase alone cannot
  group 76 WARNs by cause; `reason` distinguishes a dialed-down `SpineFloor`
  ("would-block at enforce") from a degraded digest read.
- Occurrences **accumulate** — collapsing repeats is how a 76-event epidemic
  reads as 1. A healthy cycle carries zero records and omits the JSON key, so
  the threshold alarm stays silent on a clean batch.
- Escalation keys on a cycle's OWN count (default threshold 3): a batch total
  must never drag a quiet cycle over the line.

**No behavior change to the fail-open itself** — the gate still proceeds exactly
where it did before. This slice only makes it countable.
