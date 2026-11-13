# Plan: the Signal Center measures what the pipeline delivers and detects each absent live signal

- **Status:** proposed, 2026-11-13. Revised after docs review round 1 (1 HIGH, 6 MEDIUM, 2 LOW, all folded in).
- **Design:** [signal-center-design.md](../architecture/signal-center-design.md) (the stream) and [llm-quality-indexes.md](../architecture/llm-quality-indexes.md) (the panel). This plan adds to both. **Decision:** no ADR yet; D1 to D7 go to an ADR after the design review. **Research:** §2 below.
- **Related plans:** [pool-resupply-2026-10.md](pool-resupply-2026-10.md), [wave-orchestrator-2026-10.md](wave-orchestrator-2026-10.md).

**Terms used throughout:**

- **Slot:** one of the `fleet.count` places in the pool where a lane can be live.
- **Slot utilization:** live slot-hours divided by capacity slot-hours in one wave. `evolve wave latency` computes it.
- **Idle cause:** the reason that a free slot does not take a lane.
- **Expected signal:** a signal that a stage must emit within a deadline after its start signal.
- **Delivery:** a cycle that lands a commit on `main`.
- **Calendar throughput:** deliveries for each calendar day, with the time between waves included.

## 1. Request

The operator asked on 2026-11-13:

> "Why only few lanes delivered, investigate how to design a pipeline health indexes to check the pipeline deliverables are normal and identify what can be improved"

> "Combine this request with unified signal center design"

> "Consistently detect each pipeline live signals"

§2 gives the causes of the low delivery. §4 to §8 give the design that makes the next drop visible, with its cause, in one report.

## 2. Facts

**Where the evidence lives.** The logs and run records are runtime state. They are not in git. They sit in the runtime checkout `~/ai/claude/evolve-loop/runtime/.evolve/` (`logs/`, `runs/`). The tables below copy each number with its date and its method, so the plan does not depend on those files.

### 2.1 What the loop delivered

**F2.1** Deliveries for each calendar day in November 2026. Method: count `cycle-state.json` files by the date of `started_at`, and count those with `shipped` true. Measured 2026-11-13 at about 17:30 local time.

| Day | Cycles | Deliveries |
|---|---|---|
| 11-02 | 56 | 21 |
| 11-03 | 24 | 18 |
| 11-04 | 20 | 6 |
| 11-05 | 9 | 3 |
| 11-06 | 23 | 6 |
| 11-07 | 16 | 12 |
| 11-08 | 10 | 7 |
| 11-09 | 25 | 18 |
| 11-10 | 22 | 14 |
| 11-11 | 24 | 22 |
| 11-12 | 8 | 4 |
| 11-13 (part) | 13 | 5 |

*Implication:* calendar throughput fell from 14 to 22 a day (11-09 to 11-11) to 4 or 5 a day (11-12 and 11-13). The days 11-04 to 11-06 had a similar low.

**F2.2** Waves 150 to 169. Method: `evolve wave latency --wave N --json` for the cycles, the slot utilization and the wall hours, and `shipped` in each `cycle-state.json`. Wave 153 has no latency record. Measured 2026-11-13.

| Wave | Cycles | Deliveries | Slot utilization | Wall hours | Deliveries for each wall hour |
|---|---|---|---|---|---|
| 150 | 12 | 7 | 64% | 8.3 | 0.84 |
| 151 | 15 | 14 | 98% | 8.3 | 1.69 |
| 152 | 9 | 4 | 92% | 4.3 | 0.93 |
| 154 | 9 | 2 | 97% | 4.6 | 0.43 |
| 155 | 11 | 4 | 97% | 2.8 | 1.43 |
| 156 | 9 | 3 | 94% | 14.7 | 0.20 |
| 157 | 16 | 4 | 98% | 5.8 | 0.69 |
| 158 | 5 | 2 | 97% | 5.4 | 0.37 |
| 159 | 2 | 0 | 90% | 0.9 | 0.00 |
| 160 | 16 | 12 | 97% | 7.0 | 1.71 |
| 161 | 10 | 7 | 96% | 8.5 | 0.82 |
| 162 | 15 | 10 | 84% | 13.1 | 0.76 |
| 163 | 16 | 14 | 82% | 8.8 | 1.59 |
| 164 | 16 | 8 | 49% | 14.5 | 0.55 |
| 165 | 33 | 14 | 49% | 12.0 | 1.17 |
| 166 | 16 | 12 | 57% | 10.3 | 1.17 |
| 167 | 4 | 1 | 29% | 1.4 | 0.71 |
| 168 | 4 | 3 | 57% | 2.9 | 1.03 |
| 169 | 4 | 1 | n/a | n/a | n/a |

*Implication:* the rate for each wall hour inside a wave did not fall. Its mean is 0.88 for waves 150 to 163 and 0.93 for waves 164 to 168.

Two other things changed. Slot utilization fell from about 95% to about 50% at wave 164. Waves became 4 cycles long at wave 167. A wave-hour rate does not see the time between waves, so it hides the fall in F2.1.

**F2.3** PR #1411 (commit `495bed6c8`, 2026-11-12 15:39) set `fleet.orchestrator.run_max_lanes` to 4 in `.evolve/policy.json`. The default is `4 × fleet.count`, 16 lanes (`go/internal/policy/fleet_orchestrator.go:83`). A run ends after `run_max_lanes` launches ([wave-operations.md](../operations/wave-operations.md), "When a run ends").

*Implication:* since wave 167 each wave launches 4 lanes, then each slot stays free until the wave ends. Then a boundary runs before the next wave. The operator meant 4 concurrent lanes, not 4 launches. PR #1433 removes the key.

**F2.4** The 2026-11-13 console session stopped the loop for the paste-wedge fix (PR #1430) and its live test. No wave ran for some hours of that day.
*Implication:* a part of the low 11-13 count is console downtime, not a loop fault. A calendar index must show downtime as its own cause.

**F2.5** The cause of the utilization fall at wave 164 is not known. Waves 164 to 166 ran before PR #1411. In the logs of 2026-11-10 and 2026-11-11, `LOOP_ORCH_POOL_UNDERFILLED` fired 14 to 16 times for each wave with `ready=25` to `ready=41` and no `family_held` field (`.evolve/logs/20261111T013325Z/loop.log`). A slot stayed free while 25 or more items were ready, and no field says why.
*Implication:* this is the gap that PQ8 closes. The plan does not guess the cause.

**F2.6** In waves 168 and 169, each `LOOP_ORCH_POOL_UNDERFILLED` event has `family_held=rebuild-pr-1259-derived-norm` and 3 more `rebuild-pr-*` items behind `rebuild-pr-1254-acs-compile`. Cycle 2349 (`rebuild-pr-1254-acs-compile`) sealed FAIL.
*Implication:* one item of a serial family holds 4 or more ready items for a whole wave.

**F2.7** `evolve clihealth list` on 2026-11-13 shows `agy` and `agy-claude` benched by `usage_probe` until 2026-11-14 16:53. The loop log shows `strikes=348`, and the bench end moves forward at each probe.
*Implication:* one family is out of the chain for days. Seats fall back to `claude-tmux`.

**F2.8** The gate share of lane time is 8% to 15% in waves 151 to 160, and 27% to 32% in waves 163 to 166. Method: the sum of `gate_total` over `phases.lane` in the `wave latency` record.
*Implication:* about one third of lane time goes to gates. No index tracks it.

**F2.9** Cycles 2343 and 2348 sealed FAIL with the class `intent-malformed`. In each, the guard `HEALING_I10_VIOLATION` refused a lane edit of a frozen check, `go/acs/cycle<N>/predicates_test.go`. The item of cycle 2348 had `route=lane` with the reason "no protected surface" (`.evolve/runs/cycle-2348/failure-decision.json`).
*Implication:* the triage list of protected surfaces does not include the frozen checks. Each such item costs a failed lane.

**F2.10** Cycles 2337 and 2349 sealed FAIL at the TDD handoff floor. The new test copied a helper (`moduleDir` or `goDir`) from an older `go/acs/cycle<N>` package, and the duplicate check refused it (`.evolve/runs/cycle-2337/failure-decision.json`, `.evolve/runs/cycle-2349/failure-decision.json`).
*Implication:* the `acs` packages have no shared helper for the module path. This plan counts the class. The fix is a separate item.

### 2.2 What the current signals and indexes see

**F2.11** `evolve quality report --scope wave` on 2026-11-13 gave 45 readings and 1 breach (`QUALITY_PQ4_BREACH`, deadline rate). PQ1 ship rate over 274 cycles is [0.456, 0.573], status `ok` against the threshold 0.40.
*Implication:* the panel reported `ok` while calendar throughput fell by about 75%. PQ1 measures the share of started cycles that ship. It does not measure how many cycles start.

**F2.12** The pipeline indexes are PQ1 ship rate, PQ2 pass^k, PQ3 noise floor, PQ4 deadline and system-fail rate, and PQ5 cost ([llm-quality-indexes.md §8](../architecture/llm-quality-indexes.md#8-pipeline-indexes)). None of them measures volume, slot use, idle cause or gate time.

**F2.13** The pool already types some causes:

- `LOOP_ORCH_POOL_UNDERFILLED` carries `live`, `target`, `min_lanes`, `ready` and, for a family hold, `family_held` (`go/internal/orchestrator/pool.go:127-138`).
- `LOOP_ORCH_SUPPLY` carries six supply classes (`go/internal/orchestrator/orchestrator.go:59`).
- `LOOP_ORCH_FAMILY_PAUSED` names a bench pause (`orchestrator.go:58`).
- `LOOP_ORCH_POOL_IDLE` carries only `live` and `target`. Its text names four causes, the run cap among them (`orchestrator.go:51`).

*Implication:* the family hold is typed. The other causes are free text or absent on the idle and underfill events. No reader can count them for each free slot.

**F2.14** The Signal Center records what a producer emits ([signal-center-design.md §3](../architecture/signal-center-design.md#3-target-architecture)). It has no record of what a stage must emit. A stage that stops in silence emits nothing.
*Implication:* "consistently detect each pipeline live signal" needs a list of expected signals and a deadline for each.

**F2.15** A listener can emit inside a drain. The Center queues the event, and the running drainer delivers it in `Seq` order ([signal-center-design.md §6](../architecture/signal-center-design.md#6-center-api-and-semantics), semantics 1).
*Implication:* an expected-signal listener can emit its WARN with no change to the Center.

### 2.3 Current practice

**F2.16** Flow metrics measure throughput, work in progress and flow efficiency. In a stable system, lead time ≈ WIP / throughput (Little's Law) ([getdx.com](https://getdx.com/blog/flow-metrics/), [axify.io](https://axify.io/blog/flow-metrics)). Sources say that faster output without a stability measure gives a false picture ([businesstechnavigator.com](https://businesstechnavigator.com/blog/flow-metrics-replace-story-points-cycle-time-throughput-2026)). No standard composite "health index" exists. Vendors weight their own composites (synthesis from the same sources).

**F2.17** For coding agents, sources pair the cost of each merged change with a rate of merged work, a rollback rate and cycle time. They warn that one metric alone misleads ([warp.dev](https://www.warp.dev/articles/metrics-that-prove-ai-coding-agents-are-working), [larridin.com](https://larridin.com/blog/ai-coding-agent-sessions-never-ship)).

**F2.18** Absence detection uses a dead man's switch or a heartbeat: a check alerts when an expected signal does not arrive in its window. A plain query on an absent series returns empty, not zero, so it cannot alert on silence ([oneuptime.com](https://oneuptime.com/blog/post/2026-02-06-heartbeat-dead-man-switch-opentelemetry-pipeline/view), [PromLabs](https://training.promlabs.com/training/monitoring-and-debugging-prometheus/metrics-based-meta-monitoring/end-to-end-watchdog-alerts/)). Sources say to test the switch on a schedule.

**F2.19** A p-chart fits a pass or fail fraction with binomial limits. An individuals chart (XmR) fits one measured value for each period. The first two Western Electric rules catch most shifts. More rules raise the false alarm rate ([open-exam-prep.com](https://open-exam-prep.com/study-guides/asq-cqi/ch11/attributes-control-charts-run-rules), [metricgate.com](https://metricgate.com/blogs/control-charts-process-monitoring/)).

**F2.20** The OpenTelemetry GenAI conventions define agent spans, but spans exist only where a layer emits them. Sources do not agree that the conventions are stable ([greptime.com](https://www.greptime.com/blogs/2026-05-09-opentelemetry-genai-semantic-conventions)).
*Implication:* an outside trace stack does not detect a silent layer. The expected-signal list must live with the producer.

## 3. Goals and non-goals

| Goal | Measure |
|---|---|
| G1. A drop in calendar throughput shows with its cause in the next wave report. | A replay of 11-09 to 11-13 shows the fall and names `run_cap` and console downtime as the top causes. |
| G2. Each free slot has a counted cause. | 100% of `LOOP_ORCH_POOL_UNDERFILLED` and `LOOP_ORCH_POOL_IDLE` events carry a typed `cause`. |
| G3. Each stage that starts emits its end signal, or the Center reports the absence. | A test stops a stage before its end signal, and the Center emits `SIGNAL_EXPECTED_ABSENT` at the deadline. |
| G4. The report shows the cause of a drop next to the drop. | The wave report prints the top idle cause and the top fail class next to PQ6. |

G1, G2 and G4 answer the delivery question. G3 answers the request "consistently detect each pipeline live signal". No fact in §2 shows a silent stage as a cause of the 11-12 fall.

Non-goals:

- No composite 0 to 100 score (F2.16). Each index stays a separate reading.
- No network transport and no outside trace stack (Signal Center non-goal, F2.20).
- No listener decides a verdict, a state change or a halt ([signal-center-design.md §3](../architecture/signal-center-design.md#3-target-architecture), review 10).
- No change to the phase order, the gates or the ledger.

## 4. Candidate designs

| Design | Summary |
|---|---|
| A. Flow indexes and expected signals in the current Center | Add PQ6 to PQ9 to the panel, a typed idle `cause`, and an expected-signal registry with one listener. |
| A'. Flow indexes only | A without the registry. The registry comes later, if a silent stage causes a loss. |
| B. One composite health score | Compute one weighted number from PQ1 to PQ5 and the flow data. |
| C. Outside observability stack | Export the stream to OpenTelemetry, and alert with Prometheus rules such as `absent_over_time`. |
| D. An LLM reads the logs at each boundary | A seat reads `loop.log` and writes a health summary. |

| Criterion | A | A' | B | C | D |
|---|---|---|---|---|---|
| Finds the cause, not only the drop (G1, G4) | 3 | 3 | 1 | 2 | 2 |
| Detects a silent stage (G3) | 3 | 0 | 0 | 2 | 1 |
| Deterministic and testable (AGENTS.md rule 5) | 3 | 3 | 3 | 2 | 0 |
| Cost to build and run | 2 | 3 | 3 | 1 | 1 |

The score has no total, because the criteria do not weigh the same. C fails a hard rule: the Center has no network transport (F2.20). D puts deterministic work in a prompt. B hides the cause in one number (F2.16).

A and A' tie on the delivery goals. A wins because the operator asked for G3 in the same request, and the registry needs no change to the Center (F2.15). A is built in the order of A' first (§7), so the delivery goals do not wait for the registry.

*What changes our mind:* if the registry gives more than 2 false WARNs for each wave at `shadow`, stop at A' and retune the deadlines.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Add PQ6 to PQ9 to the panel with the fields of §5.1. | F2.1, F2.2, F2.8, F2.11, F2.12 | A wave-hour rate for PQ6: it hides the time between waves (F2.2). |
| D2 | Add a typed `cause` to `LOOP_ORCH_POOL_UNDERFILLED` and `LOOP_ORCH_POOL_IDLE`. The closed set is `family_held`, `run_cap`, `supply_empty`, `admission_refused`, `bench` and `operator_stop`. `family_held` stays as the detail field of its cause. | F2.5, F2.6, F2.13 | Parse the text: a parser is a second reader of one fact ([signal-accuracy.md §5](../architecture/signal-accuracy.md#5-one-reader-for-each-fact)). |
| D3 | Add an expected-signal registry. Each entry names a start code, an end code and a deadline key in policy. One Center listener holds the open starts. It emits `SIGNAL_EXPECTED_ABSENT` (WARN) when a deadline passes with no end. | F2.14, F2.15, F2.18, F2.20 | A timer in each producer: the producer that stops is the one that cannot report. |
| D4 | The listener checks its deadlines on each event that it receives. The loop emits one new `loop.tick` event each minute, so a check runs when no stage emits. The listener emits through the Center, so the order holds (F2.15). At boot it rebuilds the open starts from `signals.ndjson`. | F2.15 | A timer inside the listener: the Center model gives a listener no clock. A tick event keeps one source of time. |
| D5 | PQ7 uses an individuals chart (XmR) with rule 1 only: one wave outside 3 sigma. PQ6 is a trend with no breach. PQ8 breaches when one cause holds more than half of the idle slot-hours of a wave. | F2.2, F2.19 | A p-chart for PQ6: PQ6 is a count rate, not a fraction. Rule 2 (8 points): too slow on 19 waves of data. |
| D6 | `evolve quality report --scope wave` and `evolve wave status` read PQ6 to PQ9 from one function. The wave report prints the top idle cause and the top fail class next to PQ6. | G4, F2.11 | A new verb: two verbs already show these numbers. |
| D7 | Triage routes each item whose fix edits a frozen check (`go/acs/cycle*/predicates_test.go`) to the console. | F2.9 | Leave it to the lane: each such item fails a lane (cycles 2343, 2348). |

**Check of D5 on the data.** Waves 151 to 163 have a PQ7 mean of 93.6% and a mean moving range of 4.4 points. The lower limit is 93.6 − 3 × 4.4 / 1.128 ≈ 81.9%. Wave 164 at 49% is below it, so rule 1 marks it.

The calendar values of F2.1 vary too much for a 3-sigma limit: the lower limit is below 0. So PQ6 alone cannot mark the 11-12 fall. PQ8 marks it, because `run_cap` holds the idle slot-hours of each wave from 167.

Consequences:

- Good: a step like wave 164 shows as a PQ7 breach in the same wave, and PQ8 names its cause.
- Good: a silent stage becomes a WARN with a code, so `evolve wave watch` wakes on it.
- Bad: the registry deadlines need tuning. A short deadline gives false WARNs (§9).
- Bad: PQ6 moves with console downtime. The report must show downtime as a cause, or PQ6 blames the loop.

### 5.1 The new indexes

| Field | PQ6 Calendar throughput | PQ7 Slot utilization | PQ8 Idle-cause share | PQ9 Gate share |
|---|---|---|---|---|
| Formula | deliveries ÷ calendar days | live slot-hours ÷ capacity slot-hours | idle slot-hours of each cause ÷ all idle slot-hours | gate time ÷ lane time |
| Unit | deliveries for each day | % | % for each cause | % |
| Scope | day; last 7 days | wave | wave | wave |
| Source | `cycle-state.json`; downtime from the loop start and exit events | `wave latency` | `cause` on the pool events (D2), and downtime | `wave latency` |
| Start stage | `report` | `shadow` | `shadow` | `report` |
| Threshold | none; a trend | rule 1 of D5 | one cause > 50% | none until 5 waves at `report` |
| Baseline | 11-09 to 11-11: 18, 14, 22 | waves 151 to 163: 93.6% | none | waves 151 to 160: 8% to 15% |
| Game and counter | Small items ship fast. Read next to PQ1 and the inbox rank. | Long lanes keep slots busy. Read next to PQ6. | A cause hidden as another. One reader computes the cause (D2). | Fewer gates. The gates are fixed by the kernel. |

H6 updates the panel spec: the index table (PQ1 to PQ9), the code count and the boundary list.

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| H1 | Typed idle `cause` on the two pool codes (D2). Current tests of `family_held` keep their field. | `TestPool_UnderfilledEventNamesTheRunCapCause`: a pool at `run_max_lanes` emits `cause=run_cap`. | lane |
| H2 | PQ6 to PQ9 readings in `pipelinerates` (D1, §5.1). | `TestCalendarThroughput_CountsDeliveriesForEachDay` on the F2.1 fixture. | lane |
| H3 | The D5 chart for PQ7 at `shadow`. | `TestUtilizationChart_MarksWave164` on the F2.2 numbers. | lane |
| H4 | Expected-signal registry and listener (D3, D4). Deadlines from `policy.json`. | `TestExpectedSignal_AbsentEndEmitsAWarnAtTheDeadline` with an injected clock, and `TestExpectedSignal_RebuildsOpenStartsAfterARestart`. | console (Signal Center wiring) |
| H5 | One reader for the wave report and `wave status` (D6). | `TestWaveStatus_AndQualityReportShowTheSameThroughput`. | lane |
| H6 | Spec updates in [llm-quality-indexes.md](../architecture/llm-quality-indexes.md), [signal-center-design.md](../architecture/signal-center-design.md) and [signal-codes.md](../architecture/signal-codes.md). | `evolve docs ste-lint --strict`. | with each component |
| H7 | Frozen-check routing (D7). | `TestTriage_FrozenCheckFixSurfaceRoutesToConsole` on the item of cycle 2348. | console |

### 6.1 The first expected-signal pairs

| Start | End that must follow | Deadline source |
|---|---|---|
| wave start | wave end | policy |
| lane launch | lane exit | the lane deadline |
| phase start | phase outcome | the phase deadline |
| phase start | first heartbeat | policy |
| cycle seal PASS | `ship.landed` | policy |
| bench start | bench probe | the bench window |
| catalog refresh | catalog commit | policy |

## 7. Order

1. **Configuration (no code).** PR #1433 removes `run_max_lanes` (F2.3). It also sets 8 lanes for 2 days at the operator's request.
2. **H7.** It stops a known failed-lane class (F2.9).
3. **H1, then H2 and H5.** These are A'. PQ8 needs the typed cause of H1.
4. **H3.** It needs the H2 readings.
5. **H4.** It needs only the Center, so it can run in parallel with steps 3 and 4.

## 8. Verification

| Goal | Before (2026-11-13) | After |
|---|---|---|
| G1 | PQ1 `ok` while calendar throughput fell from 14 to 22 to 4 or 5 a day (F2.1, F2.11). | A replay of 11-09 to 11-13 shows the fall, with `run_cap` and downtime as top causes. |
| G2 | Only the family hold is typed (F2.13). | 100% of the two pool events carry `cause`. A repo test refuses an emit with no cause. |
| G3 | No signal for an absent end. | Both H4 tests pass. A live wave at `shadow` gives 2 or fewer false WARNs. |
| G4 | The report has no cause next to the volume. | The report prints the top idle cause and the top fail class. |
| Delivery | 4 to 5 deliveries a day (11-12, 11-13). | After step 1, calendar throughput returns to the band of 11-09 to 11-11, or PQ8 names the cause. |

## 9. Risks

| Risk | Control |
|---|---|
| A short deadline gives false `SIGNAL_EXPECTED_ABSENT`. | Each deadline comes from policy. The registry starts at `shadow` for 2 waves. |
| A reading becomes a target. | Listeners never decide (non-goals). Each index has a counter (§5.1). |
| The listener misses a start after a restart. | D4 rebuilds the open starts at boot. H4 has the restart test. |
| 8 lanes overload the host and cause boot timeouts. | PQ7 and PQ4 show it in the first wave. The 8-lane setting ends on 2026-11-15. |

## 10. Open questions

- **Q1.** Must a long bench (F2.7) count only as an idle cause (`bench`), or also as an availability reading for each family?
- **Q2.** The gate share near 30% (F2.8) needs its own study. This plan only measures it.
- **Q3.** The cause of the wave 164 fall (F2.5) stays open until H1 lands and a wave runs.
