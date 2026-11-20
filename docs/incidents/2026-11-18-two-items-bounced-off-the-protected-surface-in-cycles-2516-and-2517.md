# 2026-11-18: two items bounced off the protected surface in cycles 2516 and 2517

Related documents: the drop rule is `go/internal/phases/triage/protected_route.go`, the protected manifest is `go/internal/guards/integrity_surface.go`, and the signal is declared in `go/internal/inboxmover/lifecycle/lifecycle.go`.

## Summary

- **Impact:** lane triage dropped the item of [the ship backstop record](2026-11-18-a-new-tmux-test-turned-main-red-and-the-ship-backstop-did-not-see-it.md) and the item of [the ps record](2026-11-18-faketmux-tests-need-a-ps-binary-that-a-lane-cell-denies.md). Each item became console-manual. The evidence shows a drop and a route signal for cycles 2516 and 2517, and no more.
- **Root cause:** each item names a file on the protected surface in its `files` field. `ship-backstop-misses-tree-scan-guards` names `go/internal/repocontract/repocontract.go`. `faketmux-reaper-needs-no-ps-binary` names `go/internal/proctree/`.
- **Escape:** the evidence does not show why the two items had the lane route before triage. The drop is the designed result: triage drops a card with `protected-surface: <path>` and the host routes it to the console.
- **Fix:** none yet. Both items stay with the console. Inbox items `ship-backstop-misses-tree-scan-guards` and `faketmux-reaper-needs-no-ps-binary` are open.
- **Lesson:** an item that fixes a gate or a guard can need a protected file. Check the `files` field against the manifest when the item is filed, so that a lane cycle is not spent on the drop.

## Terms used throughout

- **Lane triage:** the triage phase of a lane cycle. It picks the items that the lane builds.
- **Protected surface:** the manifest of files that a lane may not edit.
- **Console-manual:** the state of an item that only the console may work.
- **Bounce:** the drop of an item by lane triage for a protected path, and its route to the console.

## Timeline

| Time (UTC, 2026-11-18) | Event | Evidence |
|---|---|---|
| 18:15:34.293 | `INBOX_ITEM_ROUTED_CONSOLE`: `ship-backstop-misses-tree-scan-guards` is console-manual. Lane triage of cycle 2516 dropped it: `protected-surface: go/internal/repocontract/repocontract.go`. | cycle 2516 signals |
| 18:15:34.298 | `INBOX_ITEM_ROUTED_CONSOLE`: `faketmux-reaper-needs-no-ps-binary` is console-manual. Lane triage of cycle 2517 dropped it: `protected-surface: go/internal/proctree/`. | cycle 2517 signals |

The two signals are 5 ms apart. The evidence folders of 2516 and 2517 hold no other signal of WARN level or above, except the isolation signals that every cycle emits. The start time and end time of the two cycles are not in the evidence.

## Key logs

```text
INBOX_ITEM_ROUTED_CONSOLE route-console: 'ship-backstop-misses-tree-scan-guards'
is now console-manual - lane triage (cycle 2516) dropped: protected-surface:
go/internal/repocontract/repocontract.go

INBOX_ITEM_ROUTED_CONSOLE route-console: 'faketmux-reaper-needs-no-ps-binary'
is now console-manual - lane triage (cycle 2517) dropped: protected-surface:
go/internal/proctree/
```

## Root cause

**F4.1** `go/internal/repocontract/repocontract.go` is on the protected manifest as a GateGuarantee file: it holds the pack list and the tests that the ship gate runs (`go/internal/guards/integrity_surface.go:245`).
*Implication:* the class fix for the ship backstop must change the list that the backstop runs. A lane cannot make that change.

**F4.2** `go/internal/proctree/` is on the manifest as a GuardGuarantee fragment: the process tree that the reap and kill sites read (`go/internal/guards/integrity_surface.go:226`).
*Implication:* the fix for the `ps` dependency (the build report of 2513 proposes a proctree lookup) edits a protected package. A lane cannot make that change.

**F4.3** Triage drops a card whose `files` field names a protected path. It writes `protected-surface: <path>`. The host then routes the item to the console (`go/internal/phases/triage/protected_route.go:65`).
*Implication:* the bounce is the designed route. It works as designed. Each cycle spent a lane triage run to learn what the manifest already says.

**F4.4** The evidence does not show who set the route of the two items, or why each reached lane triage.
*Implication:* the root cause of the lane route is open. This record names it in Limits.

## What changed

| Change | Pull request or commit | Regression test |
|---|---|---|
| The ship backstop runs the tree-scan guards on a ship that adds or changes a test file | inbox item `ship-backstop-misses-tree-scan-guards` | open |
| The faketmux reaper finds the command of a process without the `ps` binary | inbox item `faketmux-reaper-needs-no-ps-binary` | open |
| Both items work through the console route | the two signals above | none needed, the route is existing behavior |

## Limits

- The start and end of cycles 2516 and 2517 are not in the evidence. This record cannot give their length or cost.
- The evidence does not show why the two items had the lane route when lane triage read them. F4.4 is open.
- The evidence has no failure digest or retrospective for 2516 and 2517. They may have ended with a verdict that is not in the folder.
- The state "open" of the two inbox items comes from the request for this record. The inbox is not in git.
- This record does not say that a lane cannot build a part of either fix. The signals say only that triage found a protected path in the `files` field.

## What we learned

- An item that names a protected file in `files` is console work. Route it when you file it.
- A drop for `protected-surface` is cheap, but it uses a lane triage run. The evidence does not show what else the two cycles did.
- Keep the fix of a pipeline defect out of the lane queue until the item is split into a lane part and a console part.
