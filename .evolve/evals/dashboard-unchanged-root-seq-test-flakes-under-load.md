---
score_cap:
  - criterion: "AC1 TestServer_UnchangedRootDoesNotBumpSeq waits on a condition (channel receive / select / s.subscribe) instead of a fixed time.Sleep, keeps its two current() samples, and passes"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1722_001_UnchangedRootTestWaitsOnAConditionNotAFixedSleep$' ./acs/cycle1722/"
  - criterion: "AC2 concurrent early readers of /api/snapshot for an unchanged root all see seq 1 and seq stays 1 (racing forced refreshes must not each bump seq)"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1722_002_ConcurrentEarlyReadersPublishOneSeqForAnUnchangedRoot$' ./acs/cycle1722/"
  - criterion: "AC2 Run's startup refresh(true) after an on-demand build of the same unchanged root does not publish a second seq"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1722_003_RunStartupRefreshDoesNotRebumpAnOnDemandBuiltRoot$' ./acs/cycle1722/"
  - criterion: "AC2 the durable unit test holds for 20 race-enabled repetitions"
    max_if_missing: 7
    evidence: "cd go && go test -race -count=20 -run '^TestServer_UnchangedRootDoesNotBumpSeq$' ./internal/dashboard/"
  - criterion: "AC3 negative case: a real root change (new inbox item) is still published by the running poller with a strictly higher seq — the fix must not coalesce real changes"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1722_004_PollerStillPublishesARealRootChange$' ./acs/cycle1722/"
  - criterion: "AC4 the dashboard package is vet- and gofmt-clean and its tests stay green under go test -race -count=20 with every CPU busy (the parallel go test ./... load of the inbox acceptance)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1722_005_DashboardPackageStaysGreenUnderContention$' ./acs/cycle1722/"
---

# Eval: dashboard unchanged-root seq test flakes under load

> Pins the one-publish-per-unchanged-root contract of `go/internal/dashboard`.
> `Server.refresh` skipped its unchanged-fingerprint check whenever `force` was
> set and then bumped `seq` without checking again under the write lock. So
> `Run`'s startup `refresh(true)` and `currentEpoch`'s on-demand `refresh(true)`
> could each publish a new `seq` for the same unchanged root. Cycle 1722's
> bug-reproduction phase measured seq 12–26 after 32 concurrent early readers,
> where the expected value is 1. `TestServer_UnchangedRootDoesNotBumpSeq`
> sampled `seq` across fixed 60ms sleeps. It failed under whole-module load
> whenever the poller goroutine started after the test's first on-demand
> sample. Source incident: inbox item
> `dashboard-unchanged-root-seq-test-flakes-under-load` (cycle 1722).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| condition-wait | Unit test waits on a publication signal, not `time.Sleep` | 6/10 | `TestC1722_001` |
| concurrent-coalesce | 32 racing early readers publish exactly one seq | 8/10 | `TestC1722_002` |
| run-after-on-demand | Run's startup refresh does not re-bump an unchanged root | 8/10 | `TestC1722_003` |
| durable-repetition | Unit test passes `-race -count=20` | 7/10 | `go test -race -count=20 -run TestServer_UnchangedRootDoesNotBumpSeq` |
| real-change-negative | A genuine root change still bumps seq (the check must not merge real changes) | 7/10 | `TestC1722_004` |
| load-stability | Package tests green at `-race -count=20` under CPU contention | 6/10 | `TestC1722_005` |

## Cheapest gaming fake per criterion

- AC1: deleting both sleeps and adding no wait. `TestC1722_001` requires a
  condition wait (verified RED on that fake).
- AC2: widening the sleep. `TestC1722_002`/`003` never touch the unit test.
  They drive the production handler and `Run`, so only a real single-publish
  guarantee passes.
- AC3: never rebuilding once a snapshot exists. `TestC1722_004` times out
  waiting for the new inbox item (verified RED on that fake).
