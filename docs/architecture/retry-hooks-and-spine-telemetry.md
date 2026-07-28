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
batch* instead of degrading to WARN. Patching the two misses would not fix the
class: the next hook diverges the same way.

**Design** (`go/internal/core/retry_opts.go`). `retryOpts` is a Strategy value
enumerating every recovery hook a retry loop may run:

| hook | meaning | main | batch |
|---|---|---|---|
| `backfill` | reconstruct the artifact from `stdout.clean.txt` after an `ErrArtifactTimeout` exhaustion | ✅ | ✖ (writes the workspace; the batch runs concurrently) |
| `optionalInfraSkip` | optional, off-floor phase with an infra-shaped exhaustion degrades to WARN | ✅ | ✅ |
| `postShipObserverSkip` | best-effort post-ship Control observer's failure degrades to WARN | ✅ | ✅ |
| `shipRecovery` | route a structured `ShipError` to the advisor's recovery chain | ✅ | ✖ (rewrites the loop cursor; an evaluate batch never contains ship) |

A `nil` field means "this path does not run that hook" — divergence declared and
inspectable, rather than implicit and discoverable only by diffing two loops.
`mainDispatchRetryOpts()` is the REFERENCE set (every hook wired);
`evaluateBatchRetryOpts()` is the batch subset. `retryPhaseRunner` is the shared
retry core; `dispatchRunnerWithRetry` is now a one-line delegation to it, so the
batch path cannot keep a second hand-maintained loop. The sequential loop keeps
its own control flow (it owns aborts, ledger writes and the cursor) but consults
its hooks *through the registry*, so both paths read from one enumeration.

**Guard.** `retry_opts_parity_test.go` reflects over `retryOpts` and asserts the
field set equals the canonical hook list. A hook added to the dispatch loop
without a field here fails the table — the parity gap is now structural, not
vigilance-dependent.

## 2. `IsInfraTeardownError` — the union predicate spelled once

`(errors.Is(err, ErrArtifactTimeout) || errors.Is(err, ErrTransientBridgeFailure))`
had a single-source home (`errors.go:IsInfraTeardownError`) but was still
hand-spelled at three call sites, one as the De Morgan negation. All three now
call the helper: `orchestrator.go:optionalInfraSkip`, the sequential retry arm,
and the batch arm (the latter via the shared core). `isTransientBridgeError`
stays a **transient-ONLY component** — it must never be aliased to the union;
`TestIsTransientBridgeError_StaysTransientOnly` is the anti-widen guard, and
timeout-only sites (`failure_hook.go`, `failure_learning.go`, the backfill
trigger) are *different predicates* and were deliberately left alone.

`TestInfraTeardownUnion_SpelledExactlyOnce` parses every non-test file in
`internal/core` and counts boolean expressions joining the two sentinels.
Exactly one may exist. Because it is a uniqueness assertion, it cannot be
satisfied by adding text — only by removing duplicate spellings.

## 3. Spine fail-open telemetry

**Problem.** A width-3 batch on 2026-07-13 emitted 76 `spine not satisfied …
proceeding fail-open` WARNs to stderr and nowhere else: an epidemic with no
dashboard. This change is measurement-first — make it visible, *then* decide
whether the fix is artifact-retry, driver-bounce recovery, or an enforce flip.

**Design.** Follows the `SkippedPhases` precedent exactly (one record, one
projection) rather than inventing a new shape:

```
StateMachine.UnsatisfiedSpineAnchor   → names the FIRST unsatisfied predecessor
cycleRun.recordSpineFailOpen          → cyclestate.CycleResult.SpineFailOpens
writeCycleDossier                     → dossier.BuildOpts.SpineFailOpens
                                      → Dossier.spine_fail_opens (omitempty)
dossier.RollupSpineFailOpens          → batch Total / ByPhase / OverThresholdCycles
```

- `SpineSatisfiedUpTo` now **delegates** to `UnsatisfiedSpineAnchor`, so the gate
  and its reporter cannot disagree — a reporter that inflated the counter would
  be worse than no telemetry.
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
