---
schema: incident-record/1
id: 2026-11-28-focus-pool-test-read-the-launch-order-from-goroutine-sends
title: "A focus pool test read the launch order from goroutine sends, so the scheduler sometimes changed the result"
level: record
status: fixed
selfheal: ""
fingerprint: ""
signals: []
prs: [1857, 1872, 1873]
items: []
actions: []
effectiveness:
  metric: {code: "", filters: [], unit: "", since: ""}
  baseline: {value: 0, units: 0}
  target: 0
  checks: 0
  results: []
verified_at: ""
---

# A focus pool test read the launch order from goroutine sends, so the scheduler sometimes changed the result

## Summary

<!-- GENERATED:status BEGIN -->
- **Status:** fixed
<!-- GENERATED:status END -->

## Trigger and level

- **Trigger:** on 2026-11-28 the batch PR #1872, which tested PR #1862 alone, was red in the job `go / build + test (Go 1.27, ubuntu, every package, no gates)` of run 46085997618. One of the failures was `TestFocus_FiveLanePoolLaunchesFocusIDsFirstThenOthersInRankOrder` in `go/internal/loopwave`. PR #1862 does not touch that package. The test came from PR #1857 (R1, `--focus`), which was already on main.
- **Impact:** the red helped eject #1862 from the boundary batch. No wrong code landed. The test can fail on each later PR that runs the package under load.
- **Level:** record. The red reached CI and the boundary batch, and the cause was a defect in a test on main.

## Timeline

| Time (2026-11-28, UTC) | Event | Evidence |
|---|---|---|
| 00:52 | The Go 1.27 job of run 46085997618 reports the wrong launch order. | the failed log of run 46085997618 |
| about 01:00 | The batch verb ejects #1862 and leaves it open. The other 9 PRs are on main `049fc66d3`. | `evolve integration batch` output |
| 01:05 | The console runs the test 900 times under `-race` with `-cpu 1,2,8`, and then 2400 times under parallel load. Each run passes. | `go test -race -count=300 -cpu 1,2,8` output |
| 01:10 | The console reads `fleet/pool.go` and finds that the pool decides the order in one loop, but each lane goroutine sends to the test channel. | `go/internal/fleet/pool.go:345-349` at `049fc66d3` |
| 01:12 | A 50 ms delay before the channel send makes the old test fail on each run with the CI message. The fixed test passes. | the jitter probe, 3/3 on each side |
| 01:20 | PR #1873 ships the fix with three review seats (PASS). | `evolve ship` output |

## Key logs

```text
focus_test.go:265: launch order = [first-focus second-focus alpha-ready charlie-ready bravo-ready], want [first-focus second-focus alpha-ready bravo-ready charlie-ready]
--- FAIL: TestFocus_FiveLanePoolLaunchesFocusIDsFirstThenOthersInRankOrder (0.01s)
```

The same log also has `--- FAIL` lines for `acs/cycle10`, `acs/cycle1946` and `acs/cycle1950`, and the line `writeBuildSelfCheckArtifact 81.8%`. These packages do not exist on main. Tests that run the build floor against fixtures write these lines. They are not failures of the run.

## Root cause and contributing factors

This section describes the code at `049fc66d3`, before the fix.

- **Root cause:** the test helper `laneMeter.launch` sent each lane id on the channel `m.launched` from inside the lane goroutine (`go/internal/loopwave/pool_supply_test.go:55`). The pool starts each lane with `go func()` after it emits the launched transition (`go/internal/fleet/pool.go:345-349`). In the Go memory model, two sends from two goroutines have no order. So the receive order was the order of the scheduler, not the order of the pool.
- **Second instance:** `TestFocus_RefusedFocusIDWaitsThenTakesNextFreeLane` read its first two launches from one channel in the same way (`go/internal/loopwave/focus_test.go:294`).
- **Contributing factor:** the pool emits its decision as an event (`LOOP_ORCH_LANE_LAUNCHED`, field `todo`), but no test helper recorded these events. So the tests used the side effect of the fake launch as the oracle.

## Analysis

The pool was correct. The oracle of the test was wrong: it observed execution, not the decision. On a fast host the goroutines almost always start in dispatch order, so the defect showed only on a loaded CI runner. A stress run on the host did not reproduce it. A delay that changes the goroutine order did, and that delay is the proof that the old test depends on the scheduler.

## Approaches taken

| Approach | Result |
|---|---|
| Stress the test on the host (`-race`, `-count=300`, `-cpu 1,2,8`, then 6 parallel copies) | No failure in 3300 runs. The host is too fast to change the order. |
| Read the dispatch path in `fleet/pool.go` | Found that `emit` runs in the dispatch loop before `go func()`, so the events have the true order. |
| Inject a 50 ms delay before the channel send of one lane | The old tests fail 3/3 with the CI message. The fixed tests pass 3/3. The delay was reverted before the commit. |

## Actions

<!-- GENERATED:actions BEGIN -->
_No actions._
<!-- GENERATED:actions END -->

- Fixed by PR #1873 (branch `cl/focus-test-race`). `laneMeter.publish` records the `todo` field of each `LOOP_ORCH_LANE_LAUNCHED` and `LOOP_ORCH_LANE_BACKFILLED` event through `orchestrator.Pool.Publish`. `dispatchOrder()` returns a copy under the mutex. Both focus tests assert the dispatch order. The channel receives stay as the point of synchronisation.
- Regression tests: `go/internal/loopwave/focus_test.go` (`TestFocus_FiveLanePoolLaunchesFocusIDsFirstThenOthersInRankOrder`, `TestFocus_RefusedFocusIDWaitsThenTakesNextFreeLane`). The architecture seat ran 5 mutants of the recorder and killed 4. The survivor is equal to the original for the current tests.

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
- **Metric:** `-` (unit: `-`, since: `-`, filters: none)
- **Baseline:** 0 over 0 -(s) | **Target:** 0 | **Checks:** 0
- **Results:** none
<!-- GENERATED:effectiveness END -->

- The fix is in PR #1873, which waits for the next boundary batch.

## Lessons

- An order that a test reads from goroutine sends is the order of the scheduler. Read the order from the point where the code decides it, for example the event that the code emits before it starts the goroutine.
- When a stress run on the host does not reproduce a race, inject a delay at the suspect point. A red with the delay and a green after the fix is the proof.
- In a log of tests that run other tests, count only the failures of packages that exist. The console first blamed #1862 for the `core` coverage line, which was fixture output.
