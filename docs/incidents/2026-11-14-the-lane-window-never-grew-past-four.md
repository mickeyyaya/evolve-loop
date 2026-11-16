# 2026-11-14: a fleet of 8 ran 4 lanes because the adaptive window never grows

The operator asked: "Fix 4 lanes issue which should be 8 lanes". Then: "Again, why couldn't evo loop wave taking more tasks".

Related documents: the decision [ADR-0139](../architecture/adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md), the package note [internal-orchestrator.md](../architecture/packages/internal-orchestrator.md) and the research [wave-orchestrator-2026-10.md](../research/wave-orchestrator-2026-10.md).

## Summary

- **Impact:** a wave with a fleet of 8 lanes ran 4 lanes. Wave 173 had 8 to 11 admissible items for a window of 8, and ran only 4 at a time.
- **Root cause:** the adaptive window starts at the default `window.start` of 4. It grows only when the oversight loop counts a clean landing. No production code constructs the oversight loop, so the window never grows.
- **Effect:** the waves 171 to 173 had 12 clean landings and no `LOOP_ORCH_WINDOW` event.
- **Fix:** set `fleet.orchestrator.window` to `{start: 8, min: 4}` (PR #1465, as the fact sheet gives it). The change applies at the next wave pool.
- **Open:** wire the oversight into the loop, so that the window grows on clean landings and halves on conflicts. Design comes first.

## Terms used throughout

- **The window:** the number of lanes that the orchestrator pool admits at one time. The type is `Window`.
- **The oversight loop:** the type `Oversight` in `internal/orchestrator`. It maps events of the cycle, ship, errors and loop channels to actions on the lanes of the pool.
- **A clean landing:** a landing without a conflict. It is the signal that grows the window.
- **Admissible:** a todo item that the pool can start now. A todo that waits on a busy cell is not admissible.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-11-14 | The operator sees 4 lanes in a fleet of 8. | the request above |
| 2026-11-14 | First diagnosis: too few items. The inbox reroute fixes the supply (PR #1458). The window stays at 4. | fact sheet |
| 2026-11-14 | Second diagnosis: cell conflicts. 41 to 44 items wait on a cell. This limit is real, but it is secondary. | fact sheet |
| 2026-11-14 | Wave 173 prints "orchestrator: adaptive window (stage enforce): 4 of 8 lane(s), min 4". | loop log of wave 173 |
| 2026-11-14 | The waves 171 to 173 show 12 clean landings and zero `LOOP_ORCH_WINDOW` events. | event log of the three waves |
| 2026-11-14 | The root cause is found. The fix sets the window start to 8 (PR #1465). | this record |

## Root cause

**F1.1** The default of `window.start` is 4, and the default of `window.min` is 4 (`go/internal/policy/fleet_orchestrator.go:41-42`).
*Implication:* a fleet of 8 starts the window at 4 when the policy does not set a start.

**F1.2** The loop prints the window when it builds the pool (`go/cmd/evolve/cmd_loop_orchestrator.go:136`). Wave 173 printed "4 of 8 lane(s), min 4".
*Implication:* the pool admitted 4 lanes from the first launch.

**F1.3** The window grows by one only when `Landing.CleanLanding()` runs, and it halves when `Landing.Conflict()` runs (`go/internal/orchestrator/oversight.go:286`). The type `Oversight` calls them.
*Implication:* the oversight loop is the only path that grows the window.

**F1.4** The package note says "The loop wiring of the oversight is pending" (`docs/architecture/packages/internal-orchestrator.md:27`). The search of `go/` finds no code outside the package and its tests that constructs `Oversight`.
*Implication:* in production, no clean landing reaches the window. It stays at 4.

**F1.5** The waves 171 to 173 had 12 clean landings and no `LOOP_ORCH_WINDOW` event. Wave 173 had 8 to 11 admissible items for a window of 8.
*Implication:* the supply was enough and the landings were clean. The window did not grow, as F1.4 predicts.

**F1.6** The window is built once, at the start of the pool.
*Implication:* a change of the policy applies at the next wave pool. It cannot change a wave that runs.

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Set `fleet.orchestrator.window` to `{start: 8, min: 4}`. | F1.1, F1.4 | Raise the default in code: a default of 8 does not fit a fleet of 2. |
| D2 | Wire the oversight into the loop, after a design. | F1.3, F1.4 | Do it with D1 in one change: the wiring needs its own design, such as the order of the actions and the stage. |

## What changed

| Change | Regression test |
|---|---|
| D1: the policy change in PR #1465. The tree of this record (main) does not hold it, and `.evolve/policy.json` here has no `window` block. I cannot verify the PR here. | none yet |

The inbox reroute of PR #1458 fixed the supply. It was a real fix of an earlier misdiagnosis, and it did not change the window.

## Limits

- D1 sets a fixed start. The window still does not react to conflicts until D2 is built.
- Cell conflicts stay a limit. 41 to 44 items waited on a cell, and a larger window does not remove that wait.

## What we learned

- A library that is built but not wired runs nothing. The package note said "pending", and the loop log showed a window that never changed. A check on the events of a feature (`LOOP_ORCH_WINDOW`) finds a feature that is not wired.
- Check the number that the loop prints before you look for a cause in the supply. The line "4 of 8 lane(s), min 4" gave the answer in the first look.
- Next rule to add: a guidance rule that a built component has a wiring proof (operating policy: wiring proofs are mandatory). D2 is the work item for it.
