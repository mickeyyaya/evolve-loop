# Plan: a wave orchestrator keeps at least four lanes busy, keeps their conflicts low, and scales to a large fleet

- **Status:** proposed, 2026-10-17.
- **Design:** [wave-orchestrator](../architecture/wave-orchestrator.md). **Decision:** [ADR-0139](../architecture/adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md). **Research:** [wave-orchestrator-2026-10](../research/wave-orchestrator-2026-10.md).

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions](#5-decisions)
- [6. Components](#6-components)
- [7. Order](#7-order)
- [8. Verification](#8-verification)
- [9. Risks](#9-risks)
- [10. Open questions](#10-open-questions)

## 1. Request

The operator asked for these, in order:

| Date | Words of the operator |
|---|---|
| 2026-10-16 | "Set the max cycle per wave to 4". The operator also asked for an orchestrator that oversees every parallel cycle of a wave. |
| 2026-10-17 | "Why the wave cycle number has not configured to 4 parallel running cycle? And did we have the orchestrator oversee all parallel running cycles in the same wave which I requested yesterday" |
| 2026-10-17 | "Make sure those request in your Todo in [inbox]", "And prioritize them with high prioritization" |
| 2026-10-17 | "Document and research first and make sure it can scale to large fleet number in parallel" |
| 2026-10-17 | "I want to leverage at least 4 lanes at the same time and minimize the confliction by orchestrator design" |
| 2026-10-17 | "ultrathink to investigate console routing issue" |

The inbox holds the six items of this plan in the campaign `wave-orchestrator`. The root item is `wave-orchestrator-design-set`.

## 2. Facts

| Fact | Research id |
|---|---|
| The configured width is 3, and the scheduler is the barrier wave. | F2.1 |
| Every loop run of 2026-10-16 and 2026-10-17 shrinks the wave from 3 to 2. 23 of 25 benches name `agy-claude`, which has `claude` after it in its chains. | F2.2, F2.3 |
| The shrink counts every active bench. It does not check if the wave has a fallback for the benched family. | F2.3 |
| 0 of 230 inbox items can go to a lane. 170 of the 229 console items go there by a derivation rule, not by an explicit route. | F2.6, F2.7 |
| The lane-routable supply fell from 41 items on 2026-09-15 to 1 on 2026-10-17, while the inbox stayed at about 230 items. | F2.17 |
| A directory that only contains a protected file routes to the console. The whole-bridge fragment protects 453 of 453 files. | F2.19, F2.20 |
| Four guards stop a lane at a protected file whatever the routing: the triage breaker, the role guard, the build floor and the ship tripwire. | F2.23 |
| The newest runs plan zero lanes and repair to one. | F2.4 |
| The wave seed is the last cycle's triage, which commits 1–3 items. | F2.5 |
| The live partition is file-disjoint. The package partition exists and has no caller. | F2.8, F2.9 |
| ADR-0134 designs admission by cell, with the hub cell at capacity 1. About 139 Go lanes can be pairwise disjoint. It is not built. | F2.10 |
| A conflict that reaches landing costs about 24 minutes and 1.49 LLM phases. | F2.11 |
| A barrier wave of 4 lanes keeps about 66% of the lane time busy, and 49% at 16 lanes. | F2.13 |
| The rolling pool exists, is off, and reports nothing. | F2.14 |
| No component sees all lanes of a wave. A wave lane has no deadline. | F2.12, F2.15 |
| File-based parallel agents gain nothing. Cohesive partitions with the hubs out gain up to 2.10× in wall time. | P3.1, P3.2 |
| Uncoordinated agents conflict in 20–42% of pairs. | P3.3 |
| Package-disjointness is the production independence test at Uber scale. | P3.9 |
| A predictor clears safe pairs well (F1 0.95–0.97) but finds conflicts badly (F1 0.57–0.68). | P3.7 |

## 3. Goals and non-goals

**Goals**

- **G1:** at least 4 lanes run at once whenever the supply holds at least 4 admissible items.
- **G2:** the pairwise conflict rate of landed lanes stays at or below 5%.
- **G3:** the lane time that the wave keeps busy is at least 85% at width 4.
- **G4:** one component oversees every lane of the run. It acts on a stall, a quota wall and an overlap while lanes run.
- **G5:** the design scales to a large fleet. The spec states the cost of each step as a function of the lane count N. It states the limit of N with its cause.
- **G6:** the supply of lane work is measured before each plan, and its shortfall has a named cause.

**Non-goals**

- **Landing.** ADR-0128 and ADR-0134 own the landing queue, the tiers and the group commit. The orchestrator gives them cells and events. It does not land.
- **A new partition engine.** The orchestrator wires `PartitionGraph` and ADR-0134 S14. It does not add community detection.
- **Lanes on other hosts.** The design names the host limit (§8 of the spec). Remote lanes are a later plan.

## 4. Candidate designs

### A. Configuration only

Set `fleet.count` 4 and `scheduling` `pool`, and fix the shrink so that it counts only a family with no fallback.

- **Advantages:** two config edits and one small code fix. It removes the lost lane of F2.2 at once.
- **Weaknesses:** the supply is 0 (F2.6), so the extra slots stay empty. The file-only partition stays, so conflicts stay at the uncoordinated rate (P3.1, P3.3). Nothing oversees the lanes (F2.15).

### B. Admission first, inside the barrier wave

Wire `PartitionGraph` and ADR-0134 S14 into today's `PlanFn`. Fix the shrink. Keep the barrier.

- **Advantages:** it attacks conflicts at the source, with the production rule of Uber (P3.9). It reuses built code (F2.9).
- **Weaknesses:** the barrier keeps the 66% utilization (F2.13). The supply is still 0. No component acts while lanes run.

### C. A wave orchestrator: supply, admit, backfill, oversee

One deterministic Go component owns the run of lanes. It has five steps:

1. **Supply:** it counts the admissible items before each plan, and it names the cause of a shortfall.
2. **Admit:** it admits a todo only when no lane in flight holds an overlapping cell. This is ADR-0134 S14, with `PartitionGraph`.
3. **Backfill:** it runs the rolling pool, so a free slot takes the next admissible todo at once.
4. **Lease:** it gives each lane a lease, a heartbeat and a deadline.
5. **Oversee:** it wakes on events of the ADR-0127 channels, and it acts on stalls, quota walls and landings.

- **Advantages:**
  - It is the only design that meets G1 to G6 together.
  - Each step reuses a built part: `RunPool`, `PartitionGraph`, the event reader, `FreshenSpecs` and `clihealth`.
  - Its steps are deterministic code, so no LLM tokens go to scheduling.
  - At a large N, its admission cost grows with the cells of K todos, not with N² pairs (spec §7).
- **Weaknesses:**
  - The most work of the four designs.
  - It changes how every wave runs, so it needs a shadow stage.
  - The supply step depends on the console-routing remedy (research §2.6).

### D. A planner phase that splits a goal into N cohesive tasks

A new LLM phase at each wave reads a goal and the code graph. It writes N tasks with disjoint cells, as Co-Coder does (P3.1).

- **Advantages:** it creates supply from a goal, so it does not depend on the inbox. It follows the best measured split (P3.1).
- **Weaknesses:**
  - Scheduling becomes an LLM task, against the rule that deterministic work goes in code.
  - It costs a deep-tier phase for each wave.
  - A bad split fails a whole wave.
  - It still needs C's admission and pool to run the tasks.

### Scored comparison

Scores are 1 (worst) to 5 (best).

| Criterion (weight) | A. Config | B. Admission | C. Orchestrator | D. Planner phase |
|---|:-:|:-:|:-:|:-:|
| Realized width of at least 4, G1 (×3) | 2 | 3 | 5 | 4 |
| Low conflict rate, G2 (×3) | 2 | 4 | 5 | 4 |
| Scale to a large fleet, G5 (×2) | 1 | 3 | 5 | 3 |
| Effort and risk (×2) | 5 | 4 | 2 | 2 |
| Oversight while lanes run, G4 (×1) | 1 | 1 | 5 | 3 |
| Token cost (×1) | 4 | 4 | 4 | 2 |
| **Weighted total (out of 60)** | **29** | **40** | **53** | **39** |

**The winner is C.** It is the only design that fills four slots, keeps them busy and keeps them apart. A and B fix one cause each, and the other causes keep the width at 1–2 (research §2.5). D creates supply, but it moves deterministic scheduling into an LLM and still needs C's runtime.

**What the alternatives teach.** A's shrink fix and count change go into C as its first component (O1). B's admission rule is C's admit step (O3). D's goal split stays open as a later supply source (§10, Q3), behind the same admission.

**What changes our mind.** O3 runs in shadow first. If admission by cell costs more than 10% of the width at N = 4, the non-hub cells fall back to files. If O2a and O2b free fewer than 8 items, D moves ahead of O6. The research counts 30–35 (R1a).

**The measured outcome of O2a and O2b (2026-10-17).**

1. O2a (#921) moved 3 of 35 candidates to the lanes.
2. A strict second review kept the other 32 with the console. Each one changes gate, routing-authority, ledger, phase-config or loop-control logic.
3. O2b freed 8 items. On the O2b tree, `evolve inbox rank --list ready` showed 1 item before the change and 9 after it.
4. The 3 items of O2a and the 8 items of O2b are different items. Together, O2a and O2b freed 11 items.
5. The research estimate of 30–35 was too high. The S-A and S-B estimates counted items that a strict review keeps with the console.
6. The rule of §4 moves design D ahead of O6 only when O2a and O2b free fewer than 8 items. They freed 11, so D stays after O6 in §7.
7. On 2026-10-18, the operator kept the routing roots on scope to hold four lanes (ADR-0139, amendment of decision 5). Only the split uses containment, and a declared protected path is binding.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The orchestrator is deterministic Go in the loop process. It runs no LLM. | F2.12, F2.15, P3.14 | An LLM supervisor: each action of §9 of the spec is a rule on an event, so it needs no judgement. Its tokens also grow with N. |
| D2 | The shrink counts a bench only for a family that is the last family in some chain the wave uses. | F2.2, F2.3 | Keep the shrink: it removes a lane for a family with a fallback. |
| D3 | `fleet.count` becomes 4. `fleet.min_lanes` stays 2. | §1, F2.1 | `--max-cycles 4`: that runs four waves in sequence, not four lanes. |
| D4 | Before each plan, a supply census counts the admissible items and records the cause of each refusal. | F2.6, F2.7, F2.16 | A starvation item after K waves: it reports late and repairs nothing. |
| D5 | The console re-routes the pipeline-kind items that have no protected file (S-A), and sweeps the stale explicit routes (S-B). A directory spelling routes to the console only when it lies inside a fragment (S-C). The split also refuses a directory that holds one (amended 2026-10-18). | F2.19, F2.21, F2.22, F2.23 | A narrower manifest (S-E): each fragment needs its own review, in an amendment of ADR-0064. A countersigned ship class (S-F): it removes the guard of ADR-0064. |
| D5a | When the supply is below twice the width, the supply step proposes a split of each mixed item. The split gives a console part and a lane part, linked by `deps`. The console approves each split. | F2.17, F2.20 | A one-time repair only: the supply fell for one month (F2.17), so it falls again. An automatic split: a split decides which part is a grader, which is a console judgement. |
| D6 | Dispatch admits a todo by its package cell, with the hub cell at capacity 1 (ADR-0134 S14). | F2.9, F2.10, P3.9, P3.17 | Files only: no gain over sequential (P3.1). A learned predictor: F1 0.57–0.68 on conflicts (P3.7). |
| D7 | A todo with no files gets the cell of its declared package, or the hub cell when it names none. | F2.8, P3.4 | The id as a file: such a card never conflicts on paper. |
| D8 | The rolling pool replaces the barrier. A free slot takes the next admissible todo at once. | F2.13, F2.14 | The barrier: 66% utilization at 4 lanes. |
| D9 | Each lane holds a lease with a heartbeat and a deadline from config. Past the deadline, the lane stops and releases its item. | F2.12, P3.5, P3.14 | No deadline: one lane holds a slot without limit. A hedged copy: it costs a lane and writes `main` (P3.15). |
| D10 | The orchestrator wakes on the ADR-0127 channels `cycle`, `ship`, `errors` and `loop`. It never polls. | F2.15 | A poll loop: it misses events and costs CPU at each tick. |
| D11 | On `ship.landed`, the orchestrator frees the cells of that lane and admits the next todo. | F2.10, F2.11 | A refresh at the wave end only: the slot stays idle. |
| D12 | On a quota wall of a family with no fallback, the orchestrator stops new admissions to that family. Lanes in flight continue. | F2.2, F2.15 | A stop of the whole run: it wastes the other families. |
| D13 | Above width 4, the admission window grows by 1 for each clean landing and halves on each landing conflict. | P3.10 | A fixed width at a large N: it hides a rising conflict rate. |
| D14 | Each run records the realized width, the utilization and the pairwise conflict rate in `evolve wave status`. | P3.3, P3.8 | Ship counts only: they hide idle slots and conflicts. |
| D15 | Each component lands in shadow first, then enforce, through one stage key `fleet.orchestrator.stage`. | the stage rule of ADR-0128 and ADR-0134 | A feature flag for each part: one stage key holds the rollout. |

## 6. Components

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| O0 | This design set, and the six inbox items | `evolve docs ste-lint --strict` reports 0 | console |
| O1 | D2, D3: the shrink counts a bench only without a fallback, and `fleet.count` is 4 | `TestQuotaAwareCount_ABenchWithAFallbackKeepsTheLane` | console (protected: policy) |
| O2a | D5 S-A and S-B: the console re-routes 15–18 pipeline-kind items and sweeps 6–10 stale routes, with `evolve inbox route-lane`. The 7 under-declared items name their protected paths. | `evolve inbox rank --list ready` shows at least 20 items | console (data only) |
| O2b | D5 S-C: `IsProtectedScope` routes a directory spelling only when it lies inside a fragment | `TestIsProtectedScope_ADirectoryThatOnlyContainsAFragmentIsNotProtected` | console (protected: guards) |
| O2c | D5a: the split proposal of the supply step, and `evolve inbox split` for the console. **Built 2026-10-18:** `internal/inboxsplit` and `evolve inbox split`; O6 builds the census call of `inboxsplit.Proposals`. **Fixed 2026-10-18** after the live review of 34 splits. A lane part holds no protected path and no directory that holds one, by containment in the split only. It carries real criteria and names no console file. It waits when it only verifies. | `TestSupply_ProposesASplitOfAMixedItemWhenTheSupplyIsLow`, `TestInboxSplit_TheConsolePartKeepsEveryProtectedFile` | console (protected: inbox lifecycle) |
| O3 | D6, D7: admission by cell. **Built (shadow), 2026-10-17:** [`internal/admission`](../architecture/packages/internal-admission.md), with the conflict rule of spec §6. O3 is ADR-0134 S14, built here. The ADR-0134 plan keeps S14 only as a pointer to O3, so one lane builds it. | `TestAdmit_ATodoInABusyCellWaits`, `TestAdmit_TheGlobalCellAdmitsOne`, `TestAdmit_ATodoWithNoFilesTakesItsPackageCell` | lane |
| O4 | D8: the rolling pool is the scheduler under the orchestrator. Its transitions go to the `loop` channel. **Built (enforce only), 2026-10-17:** [`internal/orchestrator`](../architecture/packages/internal-orchestrator.md) `Pool` over the hooks of `fleet.RunPool`, with `run_max_lanes`. At `shadow` the barrier wave runs as before. The soak at N = 16 passed with no exclusion: 64 lanes, peak in-flight 16, 88.0 lanes a minute, utilization 0.91 (spec §7). | `TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce` | lane |
| O5 | D9: the lane lease, the heartbeat and the deadline. **Built, 2026-10-17:** `orchestrator.Leases`, one timer over a heap, the heartbeat, `stall_minutes` (default 30) and `deadline_floor_minutes`. The heartbeat is the freshest of the cycle signals and the phase files. A stopped lane gets one dispatch reap of its own dispatch ids. At `enforce` a deadline stops the lane, releases its claim and parks it at the second stop. The wave path holds a shadow lease (spec §8). | `TestLease_ALanePastItsDeadlineStopsAndReleasesItsItem` | lane |
| O6 | D4, D10, D11, D12: the orchestrator loop over the event channels, and the supply census. **Library built, 2026-10-18:** `orchestrator.Oversight`, `Intake` and `Census`, with `fleet.PoolConfig.Kick`, the shared `shiperr.IsLandingConflict` and `loopwave.Engine.QuotaCount`. **Pending:** the loop wiring of the oversight and of the census, `evolve wave supply`, and the read of the supply record by `evolve wave status`. The split proposal step waits for PR #959 (O2c). | `TestOrchestrator_WakesOnlyOnEvents`, `TestOrchestrator_ShipLandedFreesTheCells`, `TestSupply_RecordsTheCauseOfEachRefusal` | console (protected: loop) |
| O7 | D14: width, utilization and the conflict rate in `evolve wave status`. **Built 2026-10-17:** `internal/wavemeasure`, spec §11, with the baseline of §8. | `TestWaveStatus_ReportsTheConflictRate` | lane |
| O8 | D13: the adaptive window above width 4. **Built, 2026-10-18:** `orchestrator.Window` with `fleet.orchestrator.window.start` and `window.min`, read by `fleet.RunPool` at `enforce` only. The window bounds the conflict rate. It does not lift the serial landing limit of about 6 lanes, which needs the ADR-0134 group commit (spec §10, §12). The O6 loop must call it on each `ship.landed`. Until then, it stays at `window.start`. | `TestWindow_GrowsOnACleanLandingAndHalvesOnAConflict` | lane |
| O9 | The lane CPU budget of spec §12: each `go test` of a lane gets `-p`, `-parallel` and `GOMAXPROCS` of `max(1, host CPUs / width)`, from one package `internal/lanebudget`. A structure test refuses a `go test` argv that does not go through it. | `TestProcs_DividesTheHostAmongTheLanes`, `TestEveryGoTestArgvGoesThroughTheLaneBudget` | console (protected: ship, core floors, acssuite, commitgate) |

Each component adds its spec section and its package note. Each new package is at 100% coverage in `.cover-strict` and listed in `.apicover-enforce`.

## 7. Order

The order is O0, O1, O2a, O2b, O3, O7, O4, O5, O6, O2c, O8.

1. O0 comes first: the operator asked for the design before the code.
2. O1 gives back the lost lane at once, with the smallest change.
3. O2a and O2b restore the supply. Without supply, no other component fills a slot. O2a is data only, so it can land at the next boundary. Together they freed 11 items, not 30–35 (§4, the measured outcome).
4. O3 keeps the extra lanes apart before the pool makes them run longer together.
5. O7 adds the measures, so O4 to O8 each have a before and an after.
6. O4 and O5 change how lanes run. The lease comes with the pool, so a backfilled slot has a deadline.
7. O6 joins the parts into one loop that acts on events.
8. O2c keeps the supply up after the one-time repair of O2a runs out (F2.17).
9. O8 came when the operator raised `fleet.count` to 8. It matters above width 4, and the serial landing binds above width 6. O8 bounds the conflicts only. The ADR-0134 group commit lifts the landing limit.

The inbox holds one item for each group of components, in the campaign `wave-orchestrator`:

| Inbox item | Components |
|---|---|
| `wave-orchestrator-design-set` | O0 |
| `wave-runs-four-parallel-lanes` | O1 |
| `restore-lane-routable-supply` | O2a, O2b |
| `wire-partitiongraph-for-package-level-conflicts` | O3 |
| `wave-orchestrator-oversees-every-lane` | O6, O7 |
| `rolling-pool-backfills-lanes` | O4, O5 |
| `triage-commits-disjoint-tasks-for-every-lane` | O6 (the census), O2c |

## 8. Verification

| Goal | Measure | Before | Target |
|---|---|---|---|
| G1 | the median realized width over 3 runs, from `evolve wave status --json` | 1–2 (F2.2, F2.4). O7 measured 1 for waves 118–120 (spec §11). | ≥ 4 when the supply is ≥ 4 |
| G2 | landed pairs that needed a rebase or a re-audit, over all landed pairs | The census found 46 recoveries in 14 days (F2.11). O7 measured 0 of 1 landed lanes for each of waves 118–120 (spec §11). | ≤ 5% |
| G3 | the busy lane time over the lane slots times the run time | 0.66 at 4 lanes (simulated, F2.13). O7 measured 0.32–0.33 at a policy width of 3 for waves 118–120 (spec §11). | ≥ 0.85 at 4 lanes |
| G4 | the share of stalls and deadlines that the orchestrator acts on before the console does | 0 (F2.15) | 100% |
| G5 | the spec states the cost of each step as a function of N, and a soak with the simulated driver at N = 16 runs | none | stated, and the soak passes |
| G6 | each plan records its supply count and the cause of each refusal | none | every plan |

## 9. Risks

| Risk | Control |
|---|---|
| Admission by cell makes the width too small. | O3 runs in shadow first. Its exit needs a width cost of at most 10% (the ADR-0134 S14 exit). |
| The routing remedy lets a lane change a protected file. | O2 changes where work goes, not the four guards of F2.23. The ship tripwire stays the last check. |
| A lane part of a split item is useless without its console part. | The lane part `deps` on the console part when it only verifies or when the console passes `--lane-after-console`. It then waits until the console part lands. |
| The rolling pool runs lanes longer together, so more lanes land on a moved `main`. | Admission by cell comes first (O3). ADR-0134 owns the landing cost. |
| A deadline stops a lane that is about to pass. | The deadline comes from the 95th percentile of measured lane time. A stopped lane releases its item, and nothing is lost. |
| The event reader misses an event. | The reader reads to the end of the log after each wake (ADR-0127). A gap record names any loss. |
| More lanes use more quota. | The shrink (O1) and the quota rule (D12) keep the width inside the benches. |

## 10. Open questions

- **Q1:** what is the deadline? We propose the 95th percentile of lane time over the last 50 cycles, with a floor of 60 minutes. O5 measures it in shadow. **Measured, 2026-10-17:** over cycles 1873 to 1928 (44 lane times from `signals.ndjson`), the median lane time is 39.0 minutes and the p95 is 98.9 minutes. The deadline rule removes the longest lane first (spec §8), so the deadline is 95.1 minutes. The exit rule of spec §14 (no deadline stop of a lane that passed later) needs 3 live runs of `LOOP_ORCH_LANE_LATE_EXIT` events.
- **Q2 (resolved, 2026-10-17):** how many hub packages go into the hub cell? Four: `sysexec` (151 importers), `log` (127), `atomicwrite` (126) and `signalcenter` (122). O3 measured the transitive importer count of each of the 303 module packages. The next package is `ipcenv` with 116, and the median is 5. The default `fleet.orchestrator.hub_min_importers` is 120. The measure is in [internal-admission.md](../architecture/packages/internal-admission.md) §Findings.
- **Q3:** does a goal-split planner phase (design D) become a second supply source? We propose not until O2 is live and measured.
