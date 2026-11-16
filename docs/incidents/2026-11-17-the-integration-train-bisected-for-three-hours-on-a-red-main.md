# 2026-11-17: the integration train bisected for three hours on a red main

Related documents: the train is `go/internal/integration/batchpr.go`, the lane locator is `go/internal/orchestrator/runs.go`, and the package note [internal-orchestrator.md](../architecture/packages/internal-orchestrator.md) describes the locator.

## Summary

- **Impact:** the boundary-177 batch of 13 PRs ran for 3 h 16 min and landed one PR (#1485). The wave stayed braked for that time, and 12 green PRs did not merge.
- **Root cause 1:** a flaky test made main red. `laneCycleAbove` compared a cycle directory mtime with the lane start. On Linux the mtime can be some ms before the lane start, so the oversight dropped a landing.
- **Root cause 2:** the train does not compare a red batch with main. It started on a main whose CI was still running. It split the batch on each red, also on a red that main had too.
- **Root cause 3:** the train printed nothing for each bisect step. The console watched for a log line that never came.
- **Fix:** PR #1523 adds a one-second slack to the lane locator. The train fix is inbox item `integration-train-compares-a-red-batch-with-main-before-it-splits`.
- **Lesson:** before a batch, the console must check that the CI of main at the base is complete and green.

## Terms used throughout

- **Train:** the `evolve integration batch` verb. It composes the PRs on main, runs CI on the composed branch, and fast-forwards main when CI is green.
- **Bisect step:** one split of a red batch into two halves, each with its own CI run.
- **Lane locator:** `orchestrator.LaneCycle`. It finds the cycle directory of a lane.
- **Flake:** a test that fails for a reason other than the change under test.

## Timeline

| Time (UTC, 2026-11-16) | Event | Evidence |
|---|---|---|
| 16:36 | Main CI starts on `50646266d` (cycle 2439). It fails later on `TestOrchestratedRun_ALandingAtShadowIsRecordedAndMovesNothing`. | run 43909339329 |
| 16:37 | The console starts the batch of 13 PRs. Main CI is still running. | batch log header |
| 16:38 to 19:40 | Six batch CI runs: b1, b2, b3 and b5 red, b4 green, b6 running. Each red got one rerun before the split. | runs 43909680011 to 43941736890 |
| 18:53 | b4 is green and lands #1485 alone on main (`2752dfd0f`). | `evolve-pr 1485: integrate` |
| 18:55 | Main CI on `2752dfd0f` fails on the same oversight test. | run 43933838938 |
| 19:54 | The console finds the stall: the batch log still shows only its header. | batch log |
| 19:56 | The console stops its batch process and its watcher. It reruns the failed main job. The rerun fails on the same test. | run 43933838938, second run |
| 20:23 | PR #1523 (the locator fix) ships. Batch 1 starts with #1523 alone. | PR #1523 |

## Root cause

**F1.1** `laneCycleAbove` skips a cycle directory when `info.ModTime().Before(lane.Started)` (`go/internal/orchestrator/runs.go:122` before the fix). Linux stamps a file mtime from the coarse kernel clock, which can lag `time.Now()` by one tick.
*Implication:* a cycle directory made a few ms after the lease grant looks older than the lane. `LaneCycle` returns false, and `Oversight.landed` returns with no action and no signal.

**F1.2** The two oversight landing tests passed 300 of 300 runs on macOS. On ubuntu CI the shadow test failed on main twice, and the enforce test failed in b2 and b3.
*Implication:* the flake shows only on Linux. A local check on macOS does not find it.

**F2.1** `Merger.land` (`go/internal/integration/batchpr.go:86`) splits the batch on each red after one rerun. It does not read the CI of main at the batch base.
*Implication:* when main is red, each half is red for the same reason. The bisect runs until each PR is alone, at about 45 to 50 minutes for each step.

**F2.2** The batch started at 16:37, one minute after main CI started at 16:36.
*Implication:* the console checked that main had a CI run, not that the run was complete and green.

**F3.1** The batch verb prints its order and its size, then nothing until it ends.
*Implication:* the console watcher waited for a "bisect" line, and it did not wake for 3 hours.

**F4.1** #1488 also failed b1 and b5 on apicover. #1488 adds the constant `mainstop.SchemaV1`, and no test named it. The constant is not on main.
*Implication:* the batch had one real failure under the flake. The review of its rebuild (#1524) then found a CRITICAL design defect, so #1524 is held from this boundary.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The lane locator allows one second of file-clock slack | #1523 | `TestLaneCycle_FindsADirStampedOneCoarseTickBeforeTheLaneStart` (red before the fix), `TestLaneCycle_SkipsADirStampedTwoSecondsBeforeTheLaneStart` |
| The train checks main before it batches and before it splits, and prints each step | inbox item `integration-train-compares-a-red-batch-with-main-before-it-splits` | open |
| The oversight emits a signal for a landing it cannot locate | inbox item `oversight-landing-for-an-unlocated-lane-emits-a-signal` | open |
| The main-red stop opens only on a key that a lane can prove | inbox item `main-red-stop-provable-means-a-lane-can-prove-it` (#1524 held) | open |

## Limits

- This record was written before the fixes landed. PR #1523, its two `TestLaneCycle_*` tests, PR #1488 and PR #1524 are not on main in this checkout. The three inbox items are in the runtime inbox, not in git.

- The ratio of flake failures comes from 7 CI runs. It is not a measured rate.
- This record does not prove that the coarse clock is the only cause of the flake. The red-first test reproduces the mechanism, and the fix removes it.

## What we learned

- Check that the CI of main at the base is complete and green before a batch.
- A bisect on a red main tests nothing. The train must compare a red batch with main before it splits.
- A long verb must print its progress, so that a watcher can wake on a step.
- Do not compare a file mtime with a process clock without a slack.
