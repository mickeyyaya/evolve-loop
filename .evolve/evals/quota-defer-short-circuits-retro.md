---
score_cap:
  - criterion: "An all-families-quota-exhausted (DEFERRED, rc=5 resumable) dispatch never calls the retro runner — the production RunCycle chain, not a helper unit test"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro$' ./internal/core | grep -q -- '--- PASS: TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro'"
  - criterion: "CycleState.Phase / ActiveAgent / CompletedPhases are never mutated or persisted as retro on that path, so `evolve loop --resume` re-enters the exhausted phase"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -run '^TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState$' ./internal/core | grep -q -- '--- PASS: TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState'"
  - criterion: "The short-circuit is keyed on the typed sentinel via errors.Is and still fires through multiple %w wrappers (not an identity comparison), with state.FailedAt bookkeeping recorded first"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -run '^TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro$' ./internal/core | grep -q -- '--- PASS: TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro'"
  - criterion: "NEGATIVE — a genuine non-quota dispatch failure still dispatches retro exactly once (the guard must not short-circuit every failure)"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestRunCycle_NonQuotaDispatchFailure_StillDispatchesRetroOnce$' ./internal/core | grep -q -- '--- PASS: TestRunCycle_NonQuotaDispatchFailure_StillDispatchesRetroOnce'"
  - criterion: "The guard lives in the single recordFailureLearning chokepoint that every all-families-exhausted call site already funnels through, not scattered per call site"
    max_if_missing: 6
    evidence: "cd go && test \"$(grep -c 'ErrAllFamiliesExhausted' internal/core/failure_learning.go)\" -ge 1 && go test -count=1 -run '^TestRunCycle_AllFamiliesExhausted' ./internal/core"
  - criterion: "The RED contract file is git-tracked, so the acceptance tests survive the ship commit instead of being dropped as an untracked working-tree file"
    max_if_missing: 7
    evidence: "git ls-files --error-unmatch go/internal/core/quota_defer_retro_skip_test.go"
---

# Eval: quota deferral must short-circuit the retro dispatch

> Cycle 1582 died at `scout` with every CLI family returning exit=85. That path is
> deliberately DEFERRED, not FAILED: `cyclerun_dispatch.go:264-287` writes a quota-boundary
> checkpoint, appends the `all_families_exhausted` ledger entry, and aborts with the typed
> `ErrAllFamiliesExhausted` sentinel so the loop exits rc=5 and `evolve loop --resume` can
> pick the cycle back up after the quota window resets. But the same branch then calls
> `cr.recordFailureLearning(next, phaseErr, attempt)`, and the guard at
> `failure_learning.go:344` short-circuits only for `fl.Failed == PhaseRetro` — it has no
> clause for the sentinel. So the DEFERRED path fell straight through to the retro dispatch:
> it mutated and persisted `CycleState.Phase`/`ActiveAgent` to `retro`, then ran a whole
> retro phase against the very quota wall that had just drained every family. That burns
> budget that is not there, delays the rc=5 deferral, and leaves the checkpointed cycle
> state pointing at `retro` instead of the exhausted phase.
>
> This eval pins the fix at the single chokepoint every such call site already funnels
> through, and pins its blast radius in both directions: the typed sentinel must be matched
> through arbitrary `%w` wrapping (`errors.Is`, never `==`), deterministic `state.FailedAt`
> bookkeeping must still be recorded before the short-circuit, and an ordinary non-quota
> failure must still reach retro exactly once. Source incident: cycle 1582 (instinct
> `inst-L1582a-typed-quota-defer-must-short-circuit-retro`); cycle-656 D2 established the
> checkpoint-and-defer contract whose "no retro dispatched" half this restores.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| zero-dispatch | Production RunCycle chain never calls the retro runner on all-85 | 9/10 | `-run TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro` |
| resumable-state | No persisted cycle-state ever says `retro` on that path | 8/10 | `-run ..._NeverWritesRetroCycleState` |
| wrapped-sentinel | `errors.Is` through multiple wrappers + FailedAt preserved | 8/10 | `-run TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro` |
| negative-no-overreach | Ordinary failure still dispatches retro exactly once | 9/10 | `-run ..._NonQuotaDispatchFailure_StillDispatchesRetroOnce` |
| single-chokepoint | Guard sits in `recordFailureLearning`, not per call site | 6/10 | `grep` + `-run TestRunCycle_AllFamiliesExhausted` |
| contract-tracked | The RED contract file is committed, not untracked | 7/10 | `git ls-files --error-unmatch` |

## Acceptance Criteria (code-graded)

### AC1: all-families-exhausted dispatch calls the retro runner zero times [code]
```bash
cd go && go test -count=1 -v -run '^TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro$' ./internal/core | grep -q -- '--- PASS: TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro'
```
Expected: exit 0

### AC2: no persisted cycle-state on that path names retro [code]
```bash
cd go && go test -count=1 -v -run '^TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState$' ./internal/core | grep -q -- '--- PASS: TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState'
```
Expected: exit 0

### AC3 (edge): a multiply-wrapped sentinel is still matched, bookkeeping survives [code]
```bash
cd go && go test -count=1 -v -run '^TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro$' ./internal/core | grep -q -- '--- PASS: TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro'
```
Expected: exit 0

### AC4 (negative): an ordinary non-quota failure still dispatches retro exactly once [code]
```bash
cd go && go test -count=1 -v -run '^TestRunCycle_NonQuotaDispatchFailure_StillDispatchesRetroOnce$' ./internal/core | grep -q -- '--- PASS: TestRunCycle_NonQuotaDispatchFailure_StillDispatchesRetroOnce'
```
Expected: exit 0

### AC5: the cycle's ACS predicates are green on the fixed tree [code]
```bash
cd go && go test -tags acs -count=1 -v ./acs/cycle1585 | grep -c -- '--- PASS: TestC1585_' | grep -qx 5
```
Expected: exit 0

### AC6: the RED contract file is git-tracked (not dropped at ship) [code]
```bash
git ls-files --error-unmatch go/internal/core/quota_defer_retro_skip_test.go
```
Expected: exit 0
