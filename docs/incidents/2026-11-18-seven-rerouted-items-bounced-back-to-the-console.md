# 2026-11-18: seven of the 16 rerouted items bounced back to the console

Related documents: component K4 of [the console build-or-route plan](../plans/console-build-or-route-2026-11.md). The inbox PR of the boundary is #1551.

## Summary

- **Impact:** at the wave 178 boundary, the console moved 16 `console-manual` items to lanes. In wave 179, lane triage sent 7 of them back to the console, together with 1 item that the console filed. Each bounce sealed a cycle as `SKIPPED_UNKNOWN` after 2.1 to 2.8 LLM minutes. The 8 cycles used 19.1 LLM minutes in all (`evolve wave status`, minutes to verdict).
- **Root cause:** the K4 check read only the `files` that each item declares. The real fix surface of 4 items reaches a protected file that the item does not declare. 3 more items were stale: one fix was already on main, and two depend on work that is not merged.
- **Fix:** none needed in the run. Lane triage caught each bounce before the build. The next reroute must check the real fix surface and the freshness of each item, not only the declared files.
- **Lesson:** a protected check on declared files is a lower bound. A reroute needs the same check that lane triage runs.

## Terms used throughout

- **Reroute:** `evolve inbox route-lane`, which moves an item from a console route to the lanes.
- **Bounce:** lane triage claims a rerouted item and routes it back to the console, so the cycle has no work.
- **Fix surface:** the files that a fix of the item must change, as distinct from the files that the item declares.

## Timeline

| Time (UTC, 2026-11-17) | Event | Evidence |
|---|---|---|
| 17:24 | The console reroutes the 16 items that the curator proposed, after the operator approves K4. | `evolve inbox route-lane`, rc 0 for each |
| 18:41 to 18:50 | `evolve wave next` runs the boundary and starts wave 179. Supply shows 12 admissible items in a window of 8. | `.evolve/logs/20261117T184146Z`, `evolve wave supply` |
| 18:52 to 19:03 | Lane triage routes 8 claimed items to the console. Each cycle seals `SKIPPED_UNKNOWN`. | the file time of `.evolve/runs/cycle-<n>/no-work-handoff.json` for cycles 2468, 2469, 2470, 2472, 2476, 2477, 2479 and 2480 |
| after 19:03 | The console reads the triage reasons. | the triage decision of each cycle |

## Root cause

**F1.1** The curator proposal of each reroute says "no protected fix surface and no pipeline kind explain the console route". It reads the `files` field of the item.
*Implication:* an item that declares no protected file passes, even when its fix must change one.

**F1.2** Triage named a protected fix surface for 4 rerouted items: `go/internal/core/resume_execution.go` (cycle 2470), `go/internal/phases/ship/landing/intent.go` (2477), `go/internal/bridge/driver_tmux_prepare.go` (2479) and `go/internal/commitgate/history_recorded.go` (2480).
*Implication:* the console route of these 4 items was correct. K4 moved them by mistake.

**F1.3** Triage found 3 rerouted items stale. `go/internal/bridge/driver_tmux_submitverify.go` already enforces `submitVerifyMinEchoRunes = 8` (2472). Two items depend on work that is not on main: PR #1524 (2469) and the package `go/internal/gaterun` (2476).
*Implication:* the reroute did not check if the fix is still open.

**F1.4** Cycle 2468 claimed `orchestrator-persona-decision-sections-propose-only`, an item that the console filed in the same session. Triage named the own predicate file of the cycle as a protected surface. The item changes a persona under `agents/`, so the console route is right, but the stated reason is not.
*Implication:* the triage reason for this item does not name the real cause.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The 9 rerouted items that triage kept stay on the lanes. The 7 bounced items are back on the console route that triage gave them. | none (triage routes) | none |
| The next reroute checks the real fix surface and the freshness of each item | open: amend K4 of the console build-or-route plan | open |

## What we learned

- A protected check on declared files is a lower bound. Before a reroute, check the fix surface that a fix must change. (F1.1, F1.2)
- Check that the fix of an item is still open before you move it. (F1.3)
- A bounce costs 2.1 to 2.8 LLM minutes. Lane triage is a working backstop, but not a free one. (F1.2, F1.3)
