# Plan: the pipeline sees a stalled wave, stops it, and sizes the verification capacity before it widens the fleet

- **Status:** proposed, 2026-11-21.
- **Incident:** [2026-11-21-wave-185-verification-lock-stall.md](../incidents/2026-11-21-wave-185-verification-lock-stall.md). **Retrospective:** [wave-185-verification-lock-stall.md](../retrospectives/wave-185-verification-lock-stall.md).
- **Related plans:** [fleet-host-adaptive-window-2026-11.md](fleet-host-adaptive-window-2026-11.md), [wave-orchestrator-2026-10.md](wave-orchestrator-2026-10.md).
- **Verify-lock design:** the item `verify-lock-holder-gets-the-host-budget`. Its design document is on another branch, and I do not link it.

**Terms used throughout:**

- **Host verification lock:** the one `flock` file that serializes the floor coverage run and the ACS suite. The audit integration tier has a second lock file.
- **Lane budget:** the processors that `lanebudget.Procs` gives to each `go test`: `max(1, hostCPUs/width)`.
- **Stall:** a wave with live lanes and no ship for longer than a bound.
- **Shared fault:** one fault class that recurs in several lanes of one wave.
- **Brake:** the state that stops the pool from backfill (`loopchain.BrakeEngaged`).

## 1. Request

The operator said on 2026-11-21: "Why the brake doesn't work? When we bump into consistent failure, we need to pause all the tasks and fix the blocking issue first". Then: "Write the incident report and retrospective, improvement plan". The operator asked on 2026-11-20 for 12 lanes with host-adaptive sizing, shadow first. This plan must also say what blocks that goal.

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F1 | Wave 185 shipped 0 of 9 cycles in 5.7 hours at 8 lanes. Wave 184 shipped 5 of 8 at 4 to 5 lanes. | incident record, Results |
| F2 | The lock timed out 75 times in cycles 2582 to 2590, against 6 in cycles 2575 to 2580. Each timeout is a 15 minute wait. | `runs/cycle-*/abnormal-events.jsonl` |
| F3 | The lock holder gets `max(1, hostCPUs/width)` processors, so 1 at width 8 on 14 CPUs. | `go/internal/lanebudget/lanebudget.go:48-54` |
| F4 | Three runs take a lock. The floor coverage run (`build_floor_reviewer.go:329`) and the ACS suite (`acssuite.go:159`) share one file. The audit tier uses a second file (`go/internal/ciparity/tierscope.go:13`). | code reads |
| F5 | The audit tier ran `-race -p 1 -parallel 1` and hit the 10 minute timeout in cycles 2582, 2585, 2588 and 2589. | `runs/cycle-*/audit-fail-reason.json` |
| F6 | No breaker reads a stall. The zero-ship halt runs at the wave boundary (`cmd_loop_window.go:47`). The blocker breaker reads cycle failures. The system halt needs a signal from a cycle. | code reads |
| F7 | `LOOP_ORCH_LANE_STALLED` fired 9 times with `stall_minutes=30`. It has no reader that acts. | `.evolve/signals.ndjson` |
| F8 | The host window ran at shadow with `applied=false` and flipped 10 times between 4 and 5 lanes from 10:38Z to 11:45Z. It voted 7 to 8 at 13:55:06Z. | 25 `LOOP_ORCH_HOST_PRESSURE` signals |
| F9 | The wave latency budget was exceeded in 22 waves from 159 to 183. | `loop.warning` of 08:20:16Z |
| F10 | `ClassifySeat` returns `blocked` for a live idle session. An agy seat that waits on the lock looks idle. | `go/internal/bridge/panestream/seathealth.go:39-53` |
| F11 | Outside practice: alert on the symptom a user sees and keep causes on the dashboard. Queue duration is a stall signal in CI fleets. A breaker opens when recent failures pass a threshold in a window. `-parallel` defaults to GOMAXPROCS and `-p` limits the packages that run at the same time. | section 3 |

## 3. Outside survey

**P3.1** The Google SRE book chapter on monitoring says that a monitor answers what is broken (the symptom) and why (the cause). It advises to alert on the symptom and to keep cause data on dashboards. Cause alerts fit a limit that has no symptom before it is too late, such as quota or disk. [SRE book, chapter 6](https://sre.google/sre-book/monitoring-distributed-systems/).

A search summary also gave four properties of an alert: urgent, important, actionable and real. I did not verify that wording in the book.
*Implication:* "no ship for N minutes with live lanes" is a symptom alert for this fleet. The lock timeout and the lane stall are cause signals for the dashboard and for the trip reason.

**P3.2** GitLab alerts on CI job queue duration. Its runbook raises an alert when the p95 queue time passes 10 minutes, against a normal value below 2 minutes. [GitLab runbook](https://runbooks.gitlab.com/ci-runners/alerts/CiRunnersServiceQueuingQueriesDurationApdexSLOViolation/).

A GitLab incident review found about 55 hours to detect pipelines stuck in "running" with no active jobs, because no alert existed. [GitLab incident 21470](https://gitlab.com/gitlab-com/gl-infra/production/-/issues/21470). I read only the search summary of this source.
*Implication:* the wave 185 gap (3.8 hours at the console, 5.5 hours in total) is the same class. A time-based alert on queue or ship rate closes it.

**P3.3** The circuit breaker pattern opens when recent failures pass a threshold in a time window. It then waits. A half-open state tries a limited call before it closes. [Fowler](https://www.martinfowler.com/bliki/CircuitBreaker.html); [Azure architecture guide](https://learn.microsoft.com/azure/architecture/patterns/circuit-breaker).

My search found no source on a breaker for a CI fleet. The use here is synthesis.
*Implication:* the in-wave breaker counts a fault class over a window across lanes, opens by the brake, and closes by a console act. That is a half-open state with the console as the trial.

**P3.4** A mutex is a counting semaphore with one slot. A strict FIFO semaphore can let a large request at the head block small requests. A rule that lets later arrivals take free slots uses the resource more, but it can starve a large request.

The only source I found is a patent text, which I read as a snippet and not in full. [US 7,984,439](https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/7984439).
*Implication:* one lock slot with one processor is the least work-conserving choice. A lock with K slots, each with a budget of host divided by K, keeps the host full and keeps the cap on the load. This point is synthesis, and a measure must confirm it.

**P3.5** `go test -parallel` limits tests that call `t.Parallel` inside one test binary. It defaults to GOMAXPROCS. `-p` limits how many packages run at the same time. [go testflag man page](https://manpages.debian.org/unstable/golang-go/go-testflag.7.en.html); [Go issue 36741](https://golang.org/issue/36741).

I read search summaries of both sources.
*Implication:* the budget of a run is the product of `-p` and `-parallel`. The lane budget sets both to the same number, so a holder at 1 runs one test at a time. A holder with the host budget needs `-p` and `-parallel` set from the host divided by the slots.

## 4. Goals and non-goals

**Goals.**

1. A wave with live lanes and no ship stops within 60 minutes of its first stall signal. The bound is open (question 1).
2. One shared fault engages the brake without a person.
3. The verification run that holds the lock finishes in a time that is the same as at 4 lanes.
4. A width increase has a capacity measure that it passes first.

**Non-goals.** This plan does not change the audit rules or the ship gate. It does not remove the lock correctness: verification correctness outranks wave throughput ([internal-verifylock.md](../architecture/packages/internal-verifylock.md), Findings).

## 5. Candidate designs

| Design | What it is |
|---|---|
| A | Fix each mechanism: a budget for the holder, a lock-wait signal for the window, the in-wave brake, a stall watch and a capacity check. |
| B | Remove the host lock. Rely on the lane budget alone. |
| C | One verification service. Every heavy run of every lane goes to a queue and runs with the full host budget. |
| D | A lower fixed width, for example 4 or 5, and no other change. |

| Criterion (score 1 to 5, 5 is best) | A | B | C | D |
|---|---|---|---|---|
| Removes this stall at its cause | 4 | 3 | 5 | 3 |
| Sees the next stall of another cause | 5 | 1 | 2 | 1 |
| Host safety: no false RED from a loaded host | 4 | 2 | 5 | 4 |
| Throughput at width 8 or 12 | 4 | 3 | 4 | 2 |
| Size and time to land | 3 | 5 | 1 | 5 |
| Fits the 12 lane goal | 4 | 2 | 4 | 1 |
| Total | 24 | 16 | 21 | 16 |

The scores are my judgement from F1 to F11. They are not measured.

**Why A wins.** It is the only design that adds the missing control, a symptom check on the wave (F6, F7). B removes the lock waits. But the lock exists since ADR-0080, because two lanes plus the ACS run turned a 43 second suite into false REDs. B brings that failure back.

C is the best end state for capacity. It is a new service, and it needs its own design and proof. D is correct as the interim. It changes none of the controls.

**What we use of the others.** D runs now (5 lanes by policy). C stays a later option. What changes our mind: a measured wait with K slots at width 8 above the bound in two waves. Then open the design of C.

## 6. Decisions

| Id | Decision | Because | Rejected option and why |
|---|---|---|---|
| D1 | Run 5 lanes by policy until C1 lands. | F1, F2 | Stay at 8: 0 of 9 shipped. |
| D2 | Give the holder of each verification lock the host budget less a reserve for unlocked runs and LLM sessions. Consider K slots. | F3, F4, P3.4, P3.5 | Remove the lock (design B): it brings back false REDs. |
| D3 | Publish lock wait as a signal. The host window reads it as a pressure input. | F8 | Raise the CPU threshold of the window: the host is not full. |
| D4 | The in-wave breaker counts a fault class over lanes and a time window. It starts at shadow. | F6, F7, P3.3 | Lower the threshold of the wave zero-ship breaker: it runs at the wave boundary. |
| D5 | A stall trigger in the wave watch and the console: ships against wave age, and the rate of one abnormal event kind. | F7, P3.1, P3.2 | More polling by the console: the memory rule forbids it. |
| D6 | A width increase passes a capacity check first. Fleet-12 enforce needs it. | F1, F8 | A trust in the host window: it does not read the lock. |
| D7 | Seat health names a seat that waits on a lock. | F10 | A new health state without data: it needs the signal of D3. |
| D8 | One alert for the wave latency budget, with an owner and an action. | F9, P3.1 | Delete it: the budget breach is real. |
| D9 | The one-full-suite rule of the console becomes a check. | the retrospective, M3 | A stronger note: it did not stop three agents. |

## 7. Components

Each component maps to an inbox item, or says "needs an item" with a draft. The drafts are JSON files in `build/followups-w185/` of the session scratchpad. The console files them.

| Id | Component | Red test first | Item or draft |
|---|---|---|---|
| C1 | The holder gets the host budget. It covers the floor coverage run, the ACS suite and the audit tier. | with width 8 and the lock held, the `go test` argv of the holder carries the host budget and not `-p 1` | item `verify-lock-holder-gets-the-host-budget` (the tier lock is not named in its summary: add it at design review) |
| C2 | A lock-wait signal for each acquire, and the window reads it. | a shadow record shows that the window does not widen while the wait of the last window passes its bound | item `verify-lock-holder-gets-the-host-budget` |
| C3 | The in-wave brake on a shared fault. | a fixture wave whose lanes write one abnormal event kind past the bound engages the brake, and the pool launches no lane | item `in-wave-brake-trips-on-a-shared-fault` |
| C4 | A stall trigger in `evolve wave watch` and the console: ships against wave age, and the rate of one abnormal event kind. | a replay of the wave 185 log wakes the watch before 10:00Z, and a replay of wave 184 does not | needs an item: draft `wave-watch-wakes-on-a-stall` |
| C5 | Seat health names a seat that waits on the lock. | a live idle seat with a held lock file shows `waiting`, not `blocked` | item `verify-lock-holder-gets-the-host-budget` (its fix text) |
| C6 | One alert for the wave latency budget, with an owner and an action. | a replay of waves 159 to 183 gives one open alert and not 22 items | needs an item: draft `wave-latency-breach-alerts-once-with-an-owner` |
| C7 | A capacity check before a width increase, and a gate on fleet-12 enforce. | a width increase over the measured capacity is refused with the measure named | needs an item: draft `fleet-width-increase-needs-a-capacity-check` |
| C8 | The one-full-suite check of the console during a wave. | a second console full suite while a wave runs is refused with the reason | needs an item: draft `console-full-suite-guard-during-a-wave` |
| C9 | The related floor failure of cycle 2583. | a top-level test without `t.Parallel` is named, and the full suite runs 0 times | item `floor-runs-structure-guards-first-and-keeps-failure-detail` |

## 8. Order

1. D1: the 5 lane policy. It is a setting and it stops the loss now.
2. C1 and C2 together: they share one design review and one files list. They remove the cause.
3. C3 at shadow, then C4. They catch the next stall. C3 needs a replay of the wave 185 log, and C4 reads the same log.
4. C5, which needs the signal of C2.
5. C7, after C1 and C2 give a measure. Without a measure, the check has no input.
6. C6 and C8, in any order. They are small.
7. C3 to enforce, on a verified shadow record, never before.

## 9. Verification

| Goal | Measure | Before | Target |
|---|---|---|---|
| 1 | minutes from first stall signal to a brake or a wake | 288 minutes (09:09:43 to 13:58, by hand) | 60 minutes or less |
| 2 | brake engaged without a person in a replay | 0 | 1 in the replay of wave 185 |
| 3 | lock timeouts for each cycle at the same width | 8.3 (75 in 9 cycles) | at most 1, as in wave 184 (6 in 6 cycles) |
| 4 | width increase without a capacity record | allowed | refused |
| all | cycles shipped in a wave at width 8 | 0 of 9 | at or above wave 184 (5 of 8) |

Each target needs one measured wave, as the acceptance of the items says.

## 10. Risks

| Risk | Control |
|---|---|
| A holder with the host budget loads the host and brings back false REDs. | The budget leaves a reserve. C2 feeds the window. A measured wave runs before the width goes up. |
| The in-wave breaker trips on normal signal volume and stops good waves. | It starts at shadow. The acceptance replays waves 184 and earlier and requires no trip. |
| The stall trigger repeats the alert fatigue of S5. | One alert for each wave, with an owner and an action (C6). |
| K slots starve a large run (P3.4). | Open question 2. |
| The audit tier lock is a second lock and C1 misses it. | The design review of C1 lists each lock file. |

## 11. Open questions

1. What bound for "no ship with live lanes"? A start value of 60 minutes is my guess. Wave 184 and earlier waves give the real spread of the first ship.
2. How many slots K? The item names the option. It needs a measure of one holder at the host budget against two.
3. Does the audit tier timeout come only from the budget? Run the tier on a quiet host to find out.
4. Why does the host window flip between 4 and 5? It needs a hold time. This is a follow-up of the shadow data.
