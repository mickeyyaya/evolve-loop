# 2026-11-20: a parked lane never resumed after the main-red stop released

Related documents: the stop is in [2026-11-20: cycle 2558 turned main red a third time](2026-11-20-cycle-2558-turned-main-red-a-third-time.md). The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** cycle 2563 (scope `ci-macos-fixed-parallelism`) parked at ship on the main-red stop and did not resume in wave 183. It lost about 5 hours of the wave. It resumes in wave 184 from a fresh pool.
- **Resumed:** wave 184 resumed cycle 2563 first from a fresh pool (2026-11-20T20:41Z).
- **Root cause:** the pool keeps an id in `known` for ever. The paused lead todo reuses the scope id of the first launch, so `pool.resupply` skips it. `pool.settle` never removes an id from `known`.
- **Fix:** the inbox item `pool-relaunches-a-parked-lane-after-the-main-red-stop-releases` gives the lead todo of a paused scope an explicit resume mark. The red test is `TestPoolResupply_RelaunchesAParkedLeadAfterItsLaneExited`.
- **Lesson:** a test of the order of the paused plan does not prove that the pool launches the lane.

## Terms used throughout

- **Parked lane:** a lane whose ship the main-red stop paused. The cycle keeps its worktree.
- **Paused plan:** `pausedPlan` (`go/cmd/evolve/cmd_loop_pool_paused.go:19`). It puts resumable scopes first in the next pool fill.
- **Lead todo:** the first todo of a paused scope.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| 13:46:51 | The ship of cycle 2563 aborts on the stop. The pool reports `LOOP_ORCH_LANE_EXITED` for `ci-macos-fixed-parallelism`. | `signals.ndjson:11390` and `:11391` |
| 14:30:47 | The stop is released. The reason says "the parked lanes resume in the next pool fill". | `signals.ndjson:11438` (`GATE_REPO_MAIN_RED_STOP_RELEASED`) |
| a later pool fill | The pool finds 2563 resumable. | `.evolve/logs/20261120T094708Z/loop.log:3337` |
| a second later pool fill | The pool finds 2563 resumable again. | `.evolve/logs/20261120T094708Z/loop.log:3636` |

The two log lines:

```text
[resume] WARN: origin/main is 2 commit(s) past the base 45272e443d3167322002ab77d086daee80820881; resuming, and ship rebases the cycle onto main before it lands
[resume] WARN: origin/main is 3 commit(s) past the base 45272e443d3167322002ab77d086daee80820881; resuming, and ship rebases the cycle onto main before it lands
```

Each pool fill that follows reports `LOOP_ORCH_POOL_UNDERFILLED` with 5 of 8 lanes live.

## Root cause

**F1.1** `pool.add` sets `p.known[td.ID] = true` (`go/internal/fleet/pool.go:115`). `pool.resupply` skips each todo whose id is in `known` (`:127`). `pool.settle` (`:143`) clears `running` and `claimed` but never `known`.
*Implication:* after the first launch of a scope, the pool cannot take a todo with that id again.

**F1.2** `pausedPlan.lead` (`go/cmd/evolve/cmd_loop_pool_paused.go:59`) puts the paused scope first and gives it the scope id of the first launch.
*Implication:* the lead todo of the paused scope is a known id, so the pool drops it.

**F1.3** The tests of `pausedPlan` assert only the output of `order()` (for example `TestPausedPlan_AScopeThePausedCycleHoldsItselfIsPutFirst`, `go/cmd/evolve/cmd_loop_pool_paused_hold_test.go:49`).
*Implication:* the order was right, and no test followed it into the pool.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| A resume mark on the lead todo. `pool.resupply` accepts a marked todo whose id is known, when no lane for that id is running or pending. Other known ids stay deduped. | inbox item `pool-relaunches-a-parked-lane-after-the-main-red-stop-releases` | `TestPoolResupply_RelaunchesAParkedLeadAfterItsLaneExited` (owed by the item, red first) |

## Limits

- The loss of about 5 hours is the estimate of the console. The records do not give a measured value.
- The exact times of the two later pool fills are not recorded in the lines that this record cites.

## What we learned

- Test a resume end to end through the pool entry point, not through its ordering helper.
- A set that only grows is a leak of state. Say when an id leaves it.
