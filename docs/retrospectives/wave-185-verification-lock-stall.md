# Retrospective: wave 185 stalled for 5.7 hours and the pipeline and the console both missed it

- **Type:** a blameless retrospective of one wave. The folder holds only cycle records, and docs-write has no retrospective template. This file follows the incident template (summary, timeline, cause, changes, lessons) and adds the went-well and went-wrong sections of a retrospective.
- **Date:** 2026-11-21. **Facts and evidence:** [the incident record](../incidents/2026-11-21-wave-185-verification-lock-stall.md). **Work:** [the plan](../plans/wave-stall-detection-and-verification-capacity-2026-11.md).

The operator asked: "Why the brake doesn't work? When we bump into consistent failure, we need to pause all the tasks and fix the blocking issue first".

## Summary

- Wave 185 ran 8 lanes. After 5.7 hours it had shipped 0 of 9 cycles. The incident record has the numbers and the cause.
- Two controls failed. The pipeline had no breaker that sees a stall. The console had no watch that wakes on a stall.
- The answer to the question of the operator: the brake works. It stops backfill when it is engaged (`poolHalted` in `go/cmd/evolve/cmd_loop_orchestrator.go:71`). Nothing engaged it, because no signal feeds it during a wave.
- The operator found the gap at about 13:57Z. This file records what went well, what went wrong, why, and the actions.

## What went well

- **The signals were there.** `LOOP_ORCH_LANE_STALLED` fired 9 times. The abnormal events named the lock in each cycle. A later reader can rebuild the whole wave from the records, with no text log.
- **The cause was found in one session.** Once the console read the panes and the lock holder, it found the two stacked throttles, and the first item was filed at 13:52Z.
- **The action was fast.** The items were filed 2 minutes (13:52) and 1 minute (13:59) after the findings. The brake was engaged at 13:58.
- **The host window ran at shadow.** The wave had no new risk from the adaptive window, because the window changed no lane (`applied=false`).
- **The old fix held.** The lane budget kept each locked run at one processor. At about 13:50Z the console read a load of 12.55 on 14 CPUs. The host was safe. The cost was the speed of the holder.

## What went wrong

### The system

| Id | What went wrong | Evidence |
|---|---|---|
| S1 | One heavy run held the host lock and used 1 processor. Seven lanes waited 15 minutes each, again and again. | 75 lock timeouts; `-p 1 -parallel 1` in `integration-tier.log` |
| S2 | The audit tier has its own lock file and the same budget. It timed out at 10 minutes in 4 cycles with one text. | `audit-fail-reason.json` of cycles 2582, 2585, 2588, 2589 |
| S3 | No breaker read a stall, a lock timeout or the ship rate against wave age. | the escape table of the incident record |
| S4 | `LOOP_ORCH_LANE_STALLED` published 9 signals and no code acted on them. | signal log |
| S5 | The latency-wave-breach item was filed for 22 waves. Each was consumed. The wave latency budget stayed exceeded. | the `loop.warning` of 08:20:16Z |
| S6 | The host window works at shadow. It voted to widen at 13:55Z, on a host that was slow only at the lock. It also flipped between 4 and 5 lanes 10 times from 10:38Z to 11:45Z. | 25 `LOOP_ORCH_HOST_PRESSURE` signals |
| S7 | The seat health shows an agy seat that waits for the lock as `blocked`. The cause is hidden. | `ClassifySeat`, `seathealth.go:47-48` |
| S8 | The width rose from 4 to 8 and no measure said that verification can carry 8 lanes. | the width change of 2026-11-14 |

### The console

| Id | What went wrong |
|---|---|
| M1 | The background watches of the console woke on failure codes only: `ORCHESTRATOR_CYCLE_FAILED`, `BRIDGE_EXIT_ARTIFACT_TIMEOUT`, the `QUALITY_*_BREACH` codes and the lease codes. A stall has none of them. |
| M2 | At about 10:00Z it read "0 of 8 cycles shipped, load 21" and took it for long cycles. It took no action for about 3.8 hours. |
| M3 | During the live wave it ran three review agents in parallel. Each ran a full race suite, and one reported a host load of 70 to 120. The standing rule is one full console suite during a wave. |
| M4 | It raised no question when the width went to 8 without a measure of the verification capacity. |
| M5 | It first took the idle agy seats for a quota block and checked usage before it read the panes. Usage was not exhausted (agy Gemini week 25 percent, 5 hour window 83 percent). |

## Why each happened

| Id | Why | Source |
|---|---|---|
| S1, S2 | Two good rules apply to one resource. The lock serializes heavy runs. The budget divides the host by the width. At width 8 each one is right and the product is slow. | the incident record, chain 4 and 5 |
| S3 | Each breaker was built for one failure shape: a halt code, a cycle failure, a wave count. A slow wave has none of these shapes. | `cmd_loop_window.go:47`, `cmd_loop_blockerbreaker.go` |
| S4 | The stall signal was built to tell the console. The console watch did not listen to it (M1). | `internal/orchestrator/orchestrator.go:24` |
| S5 | The alert fires for each wave, never ends, and has no owner action. It trained the console to ignore the wave latency family. This is synthesis, and I did not measure it. | the inbox items of the latency-wave-breach family |
| S6 | A shadow stage records and does not act. The window reads CPU, memory and swap, so it does not see lock wait. | [fleet-host-adaptive-window-2026-11.md](../plans/fleet-host-adaptive-window-2026-11.md), "Pressure score" |
| S7 | The check reads the pane state. An agy seat runs commands as background tasks, so the pane looks idle. | `seathealth.go:39-53` |
| S8 | The width changed through a policy PR with a throughput goal and no capacity gate. | PRs #1433 and #1446 (the console fact sheet) |
| M1 | The watches were built from the failures of the past. A failure code was the only trigger that the console knew. | console fact sheet |
| M2 | A wave of long cycles and a wave with a stall look the same in the first status. The check that tells them apart is the lane stall signal and the abnormal events. The console did not run it. | `evolve wave status` output |
| M3 | The rule is a memory note and not a guard. A review agent does not know the rule. | the memory index, standing rules |
| M4, M5 | The console had no checklist for a zero-ship wave. It chose the cause that it knew best, quota. | this retrospective, synthesis |

## What we decide

| Id | Decision | Because | Rejected option and why |
|---|---|---|---|
| RD1 | Run 5 lanes until the lock fix lands. | S1, S2: wave 184 at 4 to 5 lanes shipped 5 of 8 | Stay at 8: 0 of 9 shipped. |
| RD2 | Add a breaker that reads shared faults in a wave, at shadow first. | S3, S4 | A lower threshold on the zero-ship breaker: it still runs only at the wave boundary. |
| RD3 | Wake the console on a stall, not only on a failure. | M1, M2 | A longer polling loop: the operator memory already forbids polling. |
| RD4 | Gate a width increase on a capacity measure. | S8, M4 | A trust in the host window: it does not read the lock. |
| RD5 | Turn the one-suite rule into a guard. | M3 | A stronger note in memory: the note did not stop three agents. |
| RD6 | Replace the latency-wave-breach item with one alert that has an owner and an action. | S5 | Delete the alert: the budget breach is real. |

## Action items

| Id | Action | Owner | Inbox item |
|---|---|---|---|
| A1 | The holder of the lock gets the host budget. Lock wait is a signal. The window reads it. Seat health names the wait. | the loop (lane work, design review first) | `verify-lock-holder-gets-the-host-budget` |
| A2 | An in-wave breaker engages the brake on a shared fault and routes one blocker to the console. | the loop (design review first) | `in-wave-brake-trips-on-a-shared-fault` |
| A3 | The floor runs structure guards first and keeps the failure detail (related failure of cycle 2583). | the loop | `floor-runs-structure-guards-first-and-keeps-failure-detail` |
| A4 | The wave watch and the console wake on a stall. | the console | needs an item: draft `wave-watch-wakes-on-a-stall` |
| A5 | A capacity check gates a width increase and gates fleet-12 enforce. | the console | needs an item: draft `fleet-width-increase-needs-a-capacity-check` |
| A6 | One alert for the wave latency budget, with an owner and an action. | the console | needs an item: draft `wave-latency-breach-alerts-once-with-an-owner` |
| A7 | The one-full-suite rule of the console becomes a guard. | the console | needs an item: draft `console-full-suite-guard-during-a-wave` |
| A8 | The audit integration tier takes the same host budget as the lock holder. | the loop | part of A1 (the plan, component C1) |
| A9 | Set 5 lanes by policy until A1 lands. | the console | PR [#1645](https://github.com/mickeyyaya/evolve-loop/pull/1645) |

The drafts are in the build scratchpad of this session. The console files them. Until an item is filed, its id is a proposal.

## What we did not learn

- The exact share of lane time lost to the lock. The 41 percent figure is an upper bound from the count of timeouts.
- If the audit tier timeouts come from the budget alone. The signal text names host contention. I did not run the tier on a quiet host.
- Why the host window flapped between 4 and 5. The cause is in the thresholds, and the plan treats it as a follow-up of the shadow data.
