---
score_cap:
  - criterion: "Through Phase.runNative, a REAL newly added failing test blocks before git/ship work begins and surfaces the structured repo-contract error"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestPhaseRunNative_NewlyAddedFailingTestPreventsRun$' ./internal/phases/ship"
  - criterion: "A newly added test that is honestly t.Skip-ped remains non-blocking through Phase.runNative"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRunNative_AddedSkippedTestDoesNotBlockShip$' ./internal/phases/ship"
---

# Eval: Prove the red-test floor fires on the native shipping path

> Pins the wiring-proof half of the red-first-deliverable-reds-main fix. A
> newly-added-test detector that is correct in isolation but only reachable
> from a test is dead code — the cycle-1064 manifest-gate anti-trap applied to
> this feature. `Phase.runNative` (`go/internal/phases/ship/ship.go`) is the
> SOLE production caller that must invoke the gate before any git/ship action
> (`Run(ctx, opts)`); this eval requires a test that drives that real caller
> with a REAL newly added failing test file — not a faked pack outcome — so a
> helper-only implementation cannot satisfy it.
>
> The second cap is the negative/adversarial case: an honestly `t.Skip`-ped
> newly added test (a tracked known gap, not a hidden failure) must NOT be
> classified as a violation. A naive implementation that treats any non-"pass"
> event as a failure would turn every legitimate skip into a false RED, which
> is its own production incident class.
>
> Source incident: live inbox `red-first-deliverable-reds-main`; the user
> explicitly required production-path proof, not a helper-only detector.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wiring-proof | runNative stops before git/ship on a real added red test | 9/10 | `go test -run '^TestPhaseRunNative_NewlyAddedFailingTestPreventsRun$'` |
| skip-honesty | A t.Skip-ped added test stays non-blocking | 7/10 | `go test -run '^TestRunNative_AddedSkippedTestDoesNotBlockShip$'` |
