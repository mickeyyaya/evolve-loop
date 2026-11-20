# 2026-11-20: cycle 2559 failed on a protected file after a wrong lane route

Related documents: the review that routed the item is in [2026-11-19: the console applied the backlog dispositions with two errors](2026-11-19-the-console-applied-the-backlog-dispositions-with-two-errors.md). The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** cycle 2559 built the item `bridge-test-engine-fakes-sandbox-fence` and failed with the category `intent-malformed`. Its worktree held 75 staged files of value. No lane can finish the item.
- **Root cause:** the backlog review D6 routed the item to a lane with `files[]` set to the whole directory `go/internal/bridge`. The fix needs the protected file `go/internal/bridge/driver_tmux_submitverify_guard_test.go`. Route-lane checked only the listed path.
- **Fix:** the console routed the item to the console at the boundary of wave 183. The class fix is the inbox item `lane-fail-on-a-protected-surface-routes-the-item-to-the-console`.
- **Lesson:** a directory in `files[]` hides the protected files below it. A failed lane must not release a protected-surface item back to the lane route.

## Terms used throughout

- **Protected surface:** the paths in `ProtectedSurfaceManifest` (`go/internal/guards/integrity_surface.go:37`). The role guard denies a lane write to them.
- **Intent-malformed:** a failure category. The task contract needs an action that the lane cannot do.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| 2026-11-19, 20:46 | D6 routes the item to a lane. | [the record of the backlog dispositions](2026-11-19-the-console-applied-the-backlog-dispositions-with-two-errors.md) (the time is of the D6 run, not of this item) |
| during wave 183 | Cycle 2559 fails in the build floor. | `.evolve/runs/cycle-2559/audit-fail-reason.json` |
| 12:16 | The console files the follow-up item. The item went back to status ready with route lane. | item `lane-fail-on-a-protected-surface-routes-the-item-to-the-console` |
| boundary of wave 183 | The console routes the item to the console. A mid-wave route is refused. | console note |

The floor text of cycle 2559:

```text
--- FAIL: TestBridgeTestEnginesInjectSandboxFakes
--- FAIL: TestC2559_001_NoTestEngineReachesTheRealLogShowPoll
```

The failure decision names the cause: `driver_tmux_submitverify_guard_test.go:269:9` is a protected path (`go/internal/guards/integrity_surface.go:78`), and the role guard denies the edit.

## Root cause

**F1.1** The TDD guard scans each `NewEngine` call in the bridge tests. One call is in the protected file. No lane edit can make the guard green (`disposition.json`, cycle 2559).
*Implication:* a lane cannot meet the contract of the task.

**F1.2** The item named the directory `go/internal/bridge` in `files[]`. Intake and `route-lane` check only the listed path against the manifest.
*Implication:* the route succeeded, and the lane found the gap one claim later.

**F1.3** The failure outcome released the item to ready with route lane.
*Implication:* another lane can claim the item and fail the same way.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The item is routed to the console. | boundary inbox PR | none: a data change |
| A failed cycle with a protected-path cause leaves its item console-routed. `route-lane` expands a directory in `files[]` and refuses a protected file. | inbox item `lane-fail-on-a-protected-surface-routes-the-item-to-the-console` | owed by the item: a test through the failure outcome entry point, red before the change |

## Limits

- Until the item lands, any item with a directory in `files[]` can reach a lane with a protected file below it.
- The worktree `cycle-cd3ae73e-2559` keeps the salvage. Its use is not verified in this record.

## What we learned

- Expand a directory in `files[]` before a route to a lane.
- Check a batch of lane routes with an adversarial seat before the routes, not after a failed cycle.
