# Pipeline health readings: 2026-11-15, the start of wave 174

> **Type:** research data record. It keeps the health readings of waves 172 to 174 for a later study. It makes no decision.
>
> **Raw data:** the JSON files in this folder. The runtime keeps the source records under `.evolve/runs` and `.evolve/logs`, and `evolve gc` removes them over time. This folder keeps a copy in git.
> **Index definitions:** [llm-quality-indexes](../../../architecture/llm-quality-indexes.md) and the plan [pipeline-health-indexes-2026-11](../../../plans/pipeline-health-indexes-2026-11.md).

## 1. Method

The readings come from these commands, run in the runtime checkout on 2026-11-15 at 11:10Z, after the launch of wave 174:

| File | Command |
|---|---|
| `quality-wave.json` | `evolve quality show --scope wave --json` |
| `quality-tree.json` | `evolve quality show --scope tree --json` |
| `latency-172.json`, `latency-173.json` | `evolve wave latency --wave N --json` |
| `status-174.json` | `evolve wave status --json` |

The wave 174 readings have one or two cycles only. Use them as a baseline, not as a result.

## 2. Pipeline quality (PQ) readings

| Index | Window | Value | 95% interval | Threshold | Status | n |
|---|---|---|---|---|---|---|
| PQ1 ship rate | waves 145-174 | 0.522 | 0.467-0.576 | 0.400 | ok | 322 |
| PQ1 ship rate | waves 165-174 | 0.533 | 0.439-0.624 | 0.400 | ok | 107 |
| PQ4 deadline rate | waves 145-174 | 0.084 | 0.058-0.119 | 0.200 | ok | 322 |
| PQ4 system-fail rate | waves 145-174 | 0.075 | 0.051-0.109 | 0.050 | **breach** | 322 |
| PQ4 system-fail rate | waves 165-174 | 0.037 | 0.015-0.092 | 0.050 | inconclusive | 107 |
| PQ5 LLM minutes per shipped cycle | waves 145-174 | 110.1 | - | - | ok | 168 |
| PQ5 LLM minutes per shipped cycle | waves 165-174 | 79.3 | - | - | ok | 57 |
| PQ5 audit rounds per cycle | waves 145-174 | 1.755 | - | - | ok | 322 |
| PQ5 audit rounds per cycle | waves 165-174 | 1.374 | - | - | ok | 107 |

Notes:

- The PQ4 system-fail breach is on the 30-wave window only. The 10-wave window is inconclusive at 0.037. The rate goes down, but the old waves keep the long window above 0.05.
- PQ5 cost per shipped cycle goes down from 110.1 to 79.3 LLM minutes between the two windows.

## 3. Shadow projected stops (evidence for the shadow stage contract)

| Reading | Waves 145-174 | Waves 165-174 |
|---|---|---|
| Spend-limit projected stops | 89 | 28 |
| Spend-limit stops | 0 | 0 |
| False stops | 58 | 22 |
| Handoff projected stops | 5 | 2 |

- The spend limit is at shadow, so it stopped no cycle. Its projected stops are the evidence for the retune of fix item F7 in the shadow stage contract (PR #1472, `docs/architecture/shadow-stage-contract.md`). A projected stop is a stop that the stage records at shadow and does not apply.
- The spend limit made a projected stop on cycle 2398 of wave 173. That cycle shipped (PASS, 115.2 LLM minutes). It is one more false stop.

## 4. Lane use and idle causes

| Wave | Cycles | Wall hours | Used slot hours | Capacity slot hours | Average live lanes | Utilization (PQ7) |
|---|---|---|---|---|---|---|
| 172 | 4 | 7.0 | 16.4 | 55.9 | 2.34 | 0.292 |
| 173 | 21 | 17.7 | 40.7 | 141.3 | 2.30 | 0.288 |
| 174 (start) | - | - | - | - | - | 0.625 |

The top idle cause (PQ8) of each wave, with its share of idle time:

| Wave | Top idle cause | Share | Status |
|---|---|---|---|
| 165-170 | unknown | 1.000 | breach |
| 171 | run_cap | 0.880 | breach |
| 172 | operator_stop | 1.000 | breach |
| 173 | family_held | 0.815 | breach |
| 174 (start) | family_held | 1.000 | breach |

Notes:

- Waves 172 and 173 used fewer than 30% of the lane slots. On average, about 2.3 lanes were live in a window of 8.
- In wave 173, `family_held` was the top idle cause. A family was benched for a quota, so its lanes waited. The status line of wave 174 shows the bench `agy-claude usage_probe`.
- Wave 173 ended with one long cycle for each of its last two lanes. Cycle 2382 spent 3.5 hours in ship retries. Cycle 2398 spent 3 hours in build repair. The wave boundary waited for both.

## 5. Code quality (CQ) readings

| Index | Window | Value | Threshold | Status | n |
|---|---|---|---|---|---|
| CQ1 | tree | 0.901 | 0.878 | ok | 14378 |
| CQ2 | waves 145-174 | 2.0 | 3.0 | ok | 167 |
| CQ2 | waves 165-174 | 2.0 | 3.0 | ok | 56 |

## 6. Questions for a later study

1. Does the PQ4 system-fail rate stay under 0.05 on the 10-wave window as the waves after 174 add data?
2. Does `family_held` stay the top idle cause? If yes, the bench rule or the family share is the cause of low lane use, not the window size.
3. Does the early fast-forward check (PR #1466) remove the ship-retry tail of the kind that cycle 2382 showed?
4. What is the false-stop share of the spend limit when F7 retunes it from the shipped distribution?
5. Do waves that end on one long cycle (2382, 2398) repeat? Measure the gap from the second-to-last seal to the wave end.
