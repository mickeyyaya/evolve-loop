# 2026-11-20: a mid-wave main-red landing stranded lane ships

Related documents: the main-red stop is in [2026-11-20: cycle 2558 turned main red a third time](2026-11-20-cycle-2558-turned-main-red-a-third-time.md). The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** after PR #1600 landed mid-wave, the origin main moved to `93775a479`, and the local main of the runtime stayed at `aa909f868`. Cycle 2565 passed audit in round 3 and then failed at ship with `GIT_PUSH_REJECTED`. Cycle 2568 hit the same refusal and recovered through a new audit.
- **Root cause:** `integration.GitMain.FastForward` pushes the tip to origin only. The loop syncs the local main from origin only between iterations, and `evolve sync-main` refuses while a run lease is live. A lane ship that rebinds onto the stale local main cannot push.
- **Fix:** the console fast-forwarded the local main of the runtime by hand, while the ship window queue was empty. The class fix is the inbox item `lane-ship-fast-forwards-local-main-to-origin-in-the-ship-window`.
- **Lesson:** a landing that moves origin must move the local main in the same step, or the lane ships fail. Until the item lands, each mid-wave `--main-red-fix` landing must be followed at once by the local fast-forward.

## Terms used throughout

- **Rebind:** the ship step that rebases the lane commit onto the local main before the push.
- **Ship window:** the lease that serializes the lane ships.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| 14:30 | The main-red stop is released after the landing of #1600. | `signals.ndjson:11438` |
| 14:31 | Cycle 2565 gets `SHIP_GIT_FLEET_REBASE_NEEDED`. The rebind goes onto the local main `aa909f868`. | item `lane-ship-fast-forwards-local-main-to-origin-in-the-ship-window` |
| about 14:59 | Cycle 2568 hits `GIT_PUSH_REJECTED` and recovers after a new audit. | console note |
| about 15:10 | Cycle 2565 fails with `GIT_PUSH_REJECTED`. | `.evolve/runs/cycle-2565/ship-error.json` |
| about 15:20 | The console fast-forwards the local main with `git merge --ff-only origin/main`. Local main equals origin main. | console note |

The ship error of cycle 2565:

```text
code: GIT_PUSH_REJECTED
debug: branch=main; head=a06497b32871300aeb69f5afd432a3c9b82b8824; origin_ref=93775a47986454c5b38b92a036334ba0005e7f25; repair_attempted=GIT_PUSH_REJECTED; repair_outcome=needs-reaudit; step=push
```

## Root cause

**F1.1** `GitMain.FastForward` (`go/internal/integration/gitmain.go:25`) runs `git push origin <tip>:refs/heads/main`. It does not touch the local main.
*Implication:* after a batch, origin is ahead of the local main of the runtime.

**F1.2** The loop calls `syncMainFromOriginAtWaveBoundary` (`go/cmd/evolve/cmd_loop_wavesync.go:64`) from `go/cmd/evolve/cmd_loop_window.go:91`, between iterations only. `sync-main` refuses under a live run lease.
*Implication:* in the middle of a wave nothing moves the local main.

**F1.3** The push repair returns `needs-reaudit` (`RepairNeedsReaudit`, `go/internal/phases/ship/landing/push.go:42`) when origin diverged. It does not absorb origin first. The disposition of cycle 2565 says this is the 4th recurrence, after cycles 2362, 2363 and 2368. The failure digest gives the class recurrence as 12. The two counts are not reconciled.
*Implication:* each later lane ship of the wave fails the same way.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| Hand fast-forward of the local main of the runtime | none (console action) | none: a one-time repair |
| Inside the ship window, the ship fast-forwards a local main that is a strict ancestor of origin. A diverged main stays a halt. A `GIT_PUSH_REJECTED` of this shape takes the rebind path. | inbox item `lane-ship-fast-forwards-local-main-to-origin-in-the-ship-window` | owed by the item: an origin commit lands after the audit, and the lane ship fast-forwards, rebinds, pushes and ships (red first) |

## Limits

- The item has the files `go/internal/phases/ship/landing/push.go`. That file is protected, so the console builds the item.
- Each `--main-red-fix` landing in a live wave has this risk until the item lands.

## What we learned

- After a mid-wave batch, compare the local main with origin before the next lane ship.
- A repair that returns "audit again" on an unmoved base cannot recover. It must absorb origin first.
