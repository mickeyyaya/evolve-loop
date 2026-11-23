# 2026-11-21: wave 185 ran 8 lanes for 5.7 hours and shipped nothing, and no brake saw it

The operator asked on 2026-11-21: "Why the brake doesn't work? When we bump into consistent failure, we need to pause all the tasks and fix the blocking issue first". Then: "Write the incident report and retrospective, improvement plan".

Related documents: the retrospective [wave-185-verification-lock-stall.md](../retrospectives/wave-185-verification-lock-stall.md), the plan [wave-stall-detection-and-verification-capacity-2026-11.md](../plans/wave-stall-detection-and-verification-capacity-2026-11.md), the package notes [internal-verifylock.md](../architecture/packages/internal-verifylock.md) and [internal-lanebudget.md](../architecture/packages/internal-lanebudget.md), the plan [fleet-host-adaptive-window-2026-11.md](../plans/fleet-host-adaptive-window-2026-11.md), and the older record [2026-11-14-composed-gates-loaded-the-host-to-75-at-four-lanes.md](2026-11-14-composed-gates-loaded-the-host-to-75-at-four-lanes.md).

## Summary

- **Impact:** wave 185 ran 8 lanes from 08:17:37Z. At 14:05Z `evolve wave status` printed "0 of 9 cycles shipped". Wave 184 shipped 5 of 8 cycles with 4 to 5 live lanes.
- **Root cause:** two throttles stack. One lock lets one verification run use the host. The lane budget then gives that one run `max(1, hostCPUs/width)` processors, which is 1 at width 8 on 14 CPUs. The run is slow, and each other lane waits 15 minutes at the lock.
- **Escape:** no breaker reads a stall. Each breaker reads a failure code, a wave count or a cycle failure. The signals that showed the stall had no reader that acts.
- **Cost:** 75 lock timeouts of 15 minutes each, about 18.75 lane-hours. The wave ran about 5.7 hours at 8 lanes, so about 41 percent of the lane time (estimate).
- **Fix:** at the operator's request, the console engaged the brake at 13:58Z. Three items now track the fixes. PR [#1645](https://github.com/mickeyyaya/evolve-loop/pull/1645) sets 5 lanes until the lock fix lands.

## Terms used throughout

- **Host verification lock:** the one `flock` file `evolve-verify.lock` in the git common dir. A heavy `go test` run of a lane holds it.
- **Lock wait bound:** `verifylock.MaxWait`, 15 minutes. After it, the run goes on without the lock.
- **Lane budget:** the processor count that `lanebudget.Procs` gives to each `go test` of a lane.
- **Brake:** the file state that `loopchain.BrakeEngaged` reads. It stops the pool from backfill. Running cycles finish.
- **Shadow stage:** a feature that records its decision and changes nothing.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-11-14 | The fleet width becomes 8 (PRs #1433, #1446). The lane budget already divides the host by the width. | console fact sheet |
| 2026-11-20 | The operator asks for 12 lanes with a host-adaptive window, shadow first. | console fact sheet |
| 08:17:37 | Wave 185 starts: run `20261121T081737Z`, loop pid 13795, first cycle 2582, 8 lanes. The signal `wave.started` has the time 08:26:01. | `.evolve/signals.ndjson` |
| 08:20:16 | `loop.warning`: the wave latency budget was exceeded in 22 waves, from 159 to 183. | `.evolve/signals.ndjson` |
| 08:28:51 | Host pressure at shadow: "cap 8 to 4: pressure_high", load 31.75. `applied` is false. | signal `LOOP_ORCH_HOST_PRESSURE` |
| 08:55:57 | First lock timeout: cycle 2582, ACS suite, after a 15 minute wait. | `runs/cycle-2582/abnormal-events.jsonl` |
| 09:09:43 | First `LOOP_ORCH_LANE_STALLED`. Eight more follow until 13:52:19. | `.evolve/signals.ndjson` |
| 10:00 (about) | The console reads "pool live 8, 0 of 8 cycles shipped, load 21". It takes no action. | console fact sheet |
| 10:46:06 | First of 8 audit FAIL signals with one text: the integration tier timed out after 10 minutes. | signal `ORCHESTRATOR_PHASE_VERDICT_FAIL` |
| 12:20:58 | Cycle 2583 aborts at build: the build handoff floor fails. A separate cause. | `.evolve/signals.ndjson`, `phase.aborted` |
| 13:50 (about) | The console finds the stall. The lock holder is pid 7064, a child of lane pid 36689. | console fact sheet |
| 13:52 | The item `verify-lock-holder-gets-the-host-budget` is filed. The operator chooses 5 lanes until it lands. | inbox file of 13:52:11Z |
| 13:58 | The operator asks why the brake did not work. The console engages the brake with `evolve loop-stop`. | console fact sheet |
| 13:59 | The item `in-wave-brake-trips-on-a-shared-fault` is filed. | inbox file of 13:59:33Z |
| 14:05 | `evolve wave status`: "pool live 8", "0 of 9 cycles shipped". Cycles in flight finish. | command output |

## Key logs

The lock timeout of a cycle, the event that repeats 75 times (trimmed):

```text
event_type: verify-single-flight-unavailable   source_phase: acs-suite   cycle: 2582
timestamp:  2026-11-21T08:55:57Z   severity: WARN
details: the host verification lock .../.repo.git/evolve-verify.lock was not taken
  (verifylock: no turn at the host verification lock within the 15m0s wait bound,
  so this run goes unserialized: ... context deadline exceeded)
remediation_hint: a wait-bound miss means another lane held the lock past verifylock.MaxWait
```

The shadow window of the host, one vote to widen while the lock stalled the lanes:

```text
13:55:06  LOOP_ORCH_HOST_PRESSURE  host pressure: cap 7 to 8: pressure_low
  applied=false stage=shadow cpus=14 load1=7.43 sized=8 mem_available_pct=93
14:01:21  LOOP_ORCH_HOST_PRESSURE  host pressure: cap 8 to 4: pressure_high
  applied=false stage=shadow cpus=14 load1=25.23 sized=8
```

The lane stall signal, which fired 9 times and had no reader that acts:

```text
09:09:43  LOOP_ORCH_LANE_STALLED  lane recycle-x3-archive-leaf-writer has had no event
  since 2026-11-21T08:39:43Z   stage=enforce stall_minutes=30
```

The integration tier of the audit in cycle 2589 (header lines of `runs/cycle-2589/integration-tier.log`):

```text
# attempt 1 (lane env, contended)
# go test -race -count=1 -p 1 -parallel 1 -tags integration ./cmd/evolve/... ./internal/gc/... ./internal/gittest/...
# exit: 1
# attempt 2 (serialized retake, lock wait timed out (retake unserialized))
# go test -race -count=1 -p 1 -parallel 1 -tags integration ./cmd/evolve/... ./internal/gc/... ./internal/gittest/...
# exit: 1
```

The audit verdict text that repeated in cycles 2582, 2585, 2588 and 2589 (`audit-fail-reason.json`):

```text
the integration tier (`go test -tags integration`) reported 4 offender(s) locally. ...
An exit=80 (REPL boot timeout) offender is usually host contention ...
Offenders: panic: test timed out after 10m0s; FAIL ... go/cmd/evolve  600.886s
```

The wave status at 14:05Z:

```text
wave:    185 run 20261121T081737Z
pool:    live 8
outcome: 0 of 9 cycles shipped
cost:    ... median 99.4 LLM min to verdict, spend-limit stops 0, would stops 3
last:    wave 184: 5 of 8 cycles shipped
seat:    cycle 2582 build idle (evolve-bridge-agy-r01M7YKCJ-c2582-build-...)
seat:    cycle 2589 build idle (evolve-bridge-agy-r01M7YKCJ-c2589-build-...)
```

## Results

| Measure | Wave 185 | Wave 184 | Source |
|---|---|---|---|
| Live lanes | 8 | 4 to 5 | console fact sheet |
| Cycles shipped | 0 of 9 at 14:05Z | 5 of 8 | `evolve wave status` |
| Lock timeouts, cycles 2582 to 2590 | 75 (73 when the console counted at 13:50Z) | 6 in cycles 2575 to 2580 (the console counts 2575 to 2581) | `runs/cycle-*/abnormal-events.jsonl` |
| Lock wait lost | 75 x 15 min = 18.75 lane-hours | 1.5 lane-hours | product of the count and the bound |
| Lane stall signals | 9 | not read | `.evolve/signals.ndjson` |
| Audit FAILs with the integration-tier timeout text | 8 signals (10:46 to 13:41) | not read | `.evolve/signals.ndjson` |
| Spend-limit signals (shadow) | 3 (105.2, 116.4 and 96.2 minutes against a limit of 90) | not read | `.evolve/signals.ndjson` |

Lock timeouts for each cycle of wave 185, from cycle 2582 to 2590: 9, 5, 6, 14, 11, 6, 14, 7 and 3. The two sources differ by two events, in cycles 2584 and 2589. The cycles went on after the console count. I counted the files at 14:04Z.

The share of lane time is an estimate: 18.75 lane-hours over 8 lanes x 5.7 hours. A wait at the lock is not always idle time, because a lane can wait while a seat works. The estimate is an upper bound.

## Root cause

**Symptom.** Wave 185 shipped 0 of 9 cycles in 5.7 hours at 8 lanes (`evolve wave status`, 14:05Z).

**Chain.**

1. No cycle ships. Eight cycles stop in a build or audit phase. Cycle 2583 failed on a separate cause.
2. The cycles in the audit phase fail on the integration tier. Each run of `go test -race -tags integration ./cmd/evolve/...` hits the 10 minute test timeout (`runs/cycle-2582/audit-fail-reason.json`, 601 s). The tier runs with `-p 1 -parallel 1` (`runs/cycle-2589/integration-tier.log:2`).
3. The cycles in the build phase wait for the build floor. The floor and the ACS suite wait up to 15 minutes at the host lock. Then they run without it. (Evidence 1.) The audit tier has its own lock file, and it timed out too. (Evidence 2.)
4. The holder is slow. The lane budget gives each `go test` `max(1, hostCPUs/width)` processors, with or without the lock. At 14 CPUs and width 8, the value is 1. (Evidence 3.)
5. The two throttles stack. The lock allows one heavy run at a time. The budget lets that one run use one processor. At about 13:50Z the host load was 12.55 on 14 CPUs. The host was not full. Earlier readings were higher, for example 31.75 at 08:28:51Z.
6. The budget has a good reason. On 2026-11-14 the composed gates loaded the host to 75 at four lanes. The lock exists to stop two lanes and the ACS run from loading one host. (Evidence 4.) Nobody measured the two together at width 8.
7. The width rose from 4 to 8 on 2026-11-14. Wave 184 had 4 to 5 live lanes and had few waits. Wave 185 had 8 live lanes and many.

**Evidence for the chain.**

1. `go/internal/verifylock/verifylock.go` (`MaxWait`), `go/internal/core/build_floor_reviewer.go:329` and `go/internal/acssuite/acssuite.go:159`.
2. `go/internal/ciparity/tierscope.go:13` names `integration-tier.lock`. The logs of cycles 2586 and 2589 show "lock wait timed out (retake unserialized)".
3. `go/internal/lanebudget/lanebudget.go:48-54`. The holder at 13:50Z ran `go test -p 1 -parallel 1 -count=1 -timeout 20m -tags integration` (console fact sheet).
4. [The record of 2026-11-14](2026-11-14-composed-gates-loaded-the-host-to-75-at-four-lanes.md) and [internal-verifylock.md](../architecture/packages/internal-verifylock.md).

**Contributing causes.**

- The lock degrade is fail-open by design (`verifylock.go`, `AcquireWithin`). Each timeout is a WARN event of a cycle. It does not stop the run, and no breaker reads it.
- The host-adaptive window reads load, memory and swap. It saw a host with free CPU and voted to widen at 13:55:06Z. It was at stage shadow with `applied=false`, so it changed nothing.
- The seat health call `ClassifySeat` returns `blocked` when the session is alive and not busy (`go/internal/bridge/panestream/seathealth.go:47-48`). An agy seat that waits for a background command looks idle, so the status shows the cause as a blocked seat.

**Escape.** No control watches for a stall.

| Control | What it reads | Why it did not trip |
|---|---|---|
| `LOOP_ZERO_SHIP_HALT` | whole waves, in fleet mode | It runs before the next wave (`go/cmd/evolve/cmd_loop_window.go:47`). The wave never ended. |
| `LOOP_PIPELINE_BLOCKER_HALT` | completed cycle failures | Most cycles were still in flight (`go/cmd/evolve/cmd_loop_blockerbreaker.go`). |
| `LOOP_SYSTEM_FAILURE_HALT` | a failure that a cycle signals | No cycle signalled one. |
| the lock degrade | nothing | It is fail-open by design. |
| `LOOP_ORCH_LANE_STALLED` | 30 minutes without a lane event | It publishes a signal and acts on nothing. |
| the latency-wave-breach item | the wave latency budget | The budget was exceeded in 22 waves since 159. Each item was consumed and nothing changed. |
| the console watches | failure codes | They woke on `ORCHESTRATOR_CYCLE_FAILED` and similar codes, not on a stall. |

**The console escape.** The console read "0 of 8 cycles shipped" at about 10:00Z and took it for long cycles.

It found the stall at about 13:50Z. It first took the idle agy seats for a quota block. It checked usage before it read the panes. Usage was not exhausted: agy Gemini week 25 percent, 5 hour window 83 percent. The retrospective has the detail.

**Instance fix.** The holder of the lock gets the host budget, and the lock wait becomes a signal that the window reads (item `verify-lock-holder-gets-the-host-budget`). Until it lands, the policy sets 5 lanes (PR [#1645](https://github.com/mickeyyaya/evolve-loop/pull/1645)).

**Class fix.** An in-wave breaker trips on a shared fault (item `in-wave-brake-trips-on-a-shared-fault`), and a stall watch reads ships against the wave age (the plan, component C4).

**Regression test.** Not yet written. The acceptance of each item names a red-first test through the real entry point. Neither item has landed.

## Related failure in the same wave

Cycle 2583 failed at 12:20:58Z for a separate cause. The structure guard `TestParallelDefault` failed after the last correction of the build floor. The record is in PR #1638 (I cannot read it here). The item `floor-runs-structure-guards-first-and-keeps-failure-detail` makes the floor run the structure guards first and keep the failure text. The item names the same guard failure in cycles 2466, 2508 and 2583 in three waves. I did not verify cycles 2466 and 2508.

## What changed

| Change | Item or PR | Regression test |
|---|---|---|
| The holder gets the host budget. Lock wait is a signal. The window reads it. Seat health names a seat that waits on the lock. | item `verify-lock-holder-gets-the-host-budget` (weight 0.8) | red-first test named in its acceptance: the `go test` argv of the holder carries the host budget. Not written yet. |
| An in-wave breaker engages the brake on a fault that many lanes share. | item `in-wave-brake-trips-on-a-shared-fault` (weight 0.8) | red-first test through the loop coordinator, and a replay of the wave 185 log at shadow. Not written yet. |
| The fleet runs 5 lanes until the lock fix lands. | PR [#1645](https://github.com/mickeyyaya/evolve-loop/pull/1645) | none. It is a setting. |
| The floor runs structure guards first and keeps the failure detail. | item `floor-runs-structure-guards-first-and-keeps-failure-detail`; PR #1638 | test in `build_floor_retake_test.go`, named in its acceptance. |
| Work that the plan adds. | [the plan](../plans/wave-stall-detection-and-verification-capacity-2026-11.md) | per component |

## Limits

- I did not see the lock holder at the time of the stall. The pid and the command line come from the console fact sheet.
- I did not prove that the 8 integration-tier timeouts come from the lane budget alone. The signal text names host contention as the usual cause. The log header shows `-p 1 -parallel 1` and a lock timeout in the retake. The link is likely and not proven.
- The brake at 14:05Z did not end the cycles in flight. By design, they finish.

## What we learned

- **A fix of one overload can create the next stall.** The lane budget fixed the load of 75. With the lock, it made one holder slow. Rule to add: a limit on a shared resource is measured together with each other limit on the same resource, at the planned width. The guard is the capacity check of the plan (component C7).
- **A breaker that reads only failures does not see a stall.** Ships against wave age is a symptom, and each lane wait is a cause. The guard is the in-wave breaker and the stall watch (components C3 and C4).
- **A signal that nobody reads is a log line.** `LOOP_ORCH_LANE_STALLED` fired 9 times. The guard is a reader with an action (component C4).
- **A shadow feature does not protect.** The host window voted at shadow, so it changed no lane. The rule in memory already says to prove the enforce path before a flip. Here the consequence was an unprotected wave.
- **Zero ships after one hour is not "long cycles".** The console rule becomes a check. At zero ships with all lanes live, read the lane stall signals and the abnormal events first (component C4).
- **One full suite for the console during a wave.** Three review agents ran full race suites in parallel and added load (one reported 70 to 120). The guard is the check of component C8.
