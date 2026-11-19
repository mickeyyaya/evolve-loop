# 2026-11-18: the shadow-batch overlap test depended on sleep timing

## Summary

- **Impact:** Batch PR #1563 failed CI on try 1 on `TestShadowDispatchesTheBatchSerially`. The rerun passed.
- **Duration:** One CI job failure (`run 44232820753`, `attempt 1`, `job 134561970949`) and one cycle 2492 audit continuation into cycle 2524.
- **Root cause:** `overlapRunner.Run` in `go/internal/core/evaluate_batch_shadow_test.go` used `time.Sleep(40 * time.Millisecond)` to detect overlap. Under scheduler load, the second enforce member entered `Run` after the first member left its sleep.
- **Fix:** Replace `time.Sleep` with a channel rendezvous (`entered` channel closed with `sync.Once`) with a `10*time.Second` bound in enforce.
- **Lesson:** Tests that detect concurrent execution must synchronize with a channel rendezvous, not a sleep window.

## Timeline

| Time (UTC, 2026-11-18) | Event | Evidence |
|---|---|---|
| 04:50 | Batch PR #1563 CI try 1 fails on `TestShadowDispatchesTheBatchSerially`. Rerun passes. | `run 44232820753`, `job 134561970949` |
| 04:50 | Inbox item `core-shadow-batch-overlap-test-depends-on-sleep-timing` opens. | `.evolve/inbox/processing/cycle-2524/2026-11-18T04-50-43Z-core-shadow-batch-overlap-test-depends-on-sleep-timing.json` |
| 14:56 | Cycle 2492 builds the rendezvous fix, and its audit fails on wrong-root policy parsing (`H1`). | `.evolve/runs/cycle-2492/audit-report.md` |
| 2026-11-19 01:01 | Cycle 2524 resumes from the cycle 2492 snapshot and verifies all five ACS predicates. | `.evolve/runs/cycle-2524/test-report.md` |

## Root cause

**F1.1** `overlapRunner.Run` slept `40 * time.Millisecond` after incrementing `active` (`go/internal/core/evaluate_batch_shadow_test.go:77` at `46440c64f`).
*Implication:* The `StageEnforce` control passed only when the scheduler ran both goroutines inside a 40 ms window.

**F1.2** On a loaded macOS runner, the second goroutine entered `Run` after the first goroutine exited `Run`. The `active.Add(1) > 1` check stayed false and failed the control at `evaluate_batch_shadow_test.go:100`.
*Implication:* Scheduler load turned a deterministic concurrency check into a flaky failure.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| `overlapRunner` closes its `entered` channel with `sync.Once` and waits on `<-r.peer.entered`, `<-time.After(r.wait)`, or `<-ctx.Done()` (`10*time.Second` for `StageEnforce`, `50*time.Millisecond` for `StageShadow`). | cycle 2524 (`core-shadow-batch-overlap-test-depends-on-sleep-timing`) | `go/acs/cycle2524/predicates_test.go::TestC2524_001_EnforceControlSurvivesASlowScheduler`, `::TestC2524_002_NoSleepAndRendezvousBoundAtLeastFiveSeconds`, `::TestC2524_003_ConcurrentShadowDispatchStillTurnsTheTestRed`, `::TestC2524_004_StaysGreenRepeatedlyUnderOneAndTwoProcs` |

## What we learned

| Lesson | Rule or guard it became |
|---|---|
| Never prove concurrent dispatch with `time.Sleep`. Use a channel rendezvous with a generous control bound (`>= 5s`). (F1.1, F1.2) | `go/acs/cycle2524/predicates_test.go::TestC2524_001_EnforceControlSurvivesASlowScheduler` and `::TestC2524_002_NoSleepAndRendezvousBoundAtLeastFiveSeconds` |
