# Pipeline health readings: 2026-11-16, the end of wave 174

> **Type:** research data record. It keeps the health readings of the wave 174 boundary for a later study. It makes no decision.
>
> **Series:** this is the second snapshot. The first is the 2026-11-15 snapshot (PR #1483), taken at the start of wave 174. Section 7 compares the two.
>
> **Raw data:** the JSON files in this folder. The runtime keeps the source records under `.evolve/runs` and `.evolve/logs`, and `evolve gc` removes them over time. This folder keeps a copy in git.
> **Index definitions:** [llm-quality-indexes](../../../architecture/llm-quality-indexes.md) and the plan [pipeline-health-indexes-2026-11](../../../plans/pipeline-health-indexes-2026-11.md).

## 1. Method

The readings come from these commands, run in the runtime checkout at the wave 174 boundary, on 2026-11-16 at about 03:00Z:

| File | Command |
|---|---|
| `quality-wave.json` | `evolve quality show --scope wave --json` |
| `quality-tree.json` | `evolve quality show --scope tree --json` |
| `latency-174.json` | `evolve wave latency --wave 174 --json` |
| `status-174.json` | `evolve wave status --json` |

Some facts come from the console, not from these files. This record cites them as "console boundary log 2026-11-16".

Wave 174 ran cycles 2401 to 2420. Its outcome line is "11 of 20 cycles shipped" (`status-174.json`). The wave scope of `quality-wave.json` also holds cycle 2399, so its wave 174 window has 21 cycles and 12 ships. Section 6 records this difference.

## 2. Pipeline quality (PQ) readings

| Index | Window | Value | 95% interval | Threshold | Status | n |
|---|---|---|---|---|---|---|
| PQ1 ship rate | wave 174 | 0.571 | 0.365-0.755 | 0.400 | inconclusive | 21 |
| PQ1 ship rate | waves 165-174 | 0.540 | 0.453-0.624 | 0.400 | ok | 126 |
| PQ1 ship rate | waves 145-174 | 0.525 | 0.472-0.577 | 0.400 | ok | 341 |
| PQ4 deadline rate | wave 174 | 0.000 | 0.000-0.155 | 0.200 | ok | 21 |
| PQ4 deadline rate | waves 165-174 | 0.000 | 0.000-0.030 | 0.200 | ok | 126 |
| PQ4 deadline rate | waves 145-174 | 0.079 | 0.055-0.113 | 0.200 | ok | 341 |
| PQ4 system-fail rate | wave 174 | 0.000 | 0.000-0.155 | 0.050 | inconclusive | 21 |
| PQ4 system-fail rate | waves 165-174 | 0.032 | 0.012-0.079 | 0.050 | inconclusive | 126 |
| PQ4 system-fail rate | waves 145-174 | 0.070 | 0.048-0.103 | 0.050 | inconclusive | 341 |
| PQ5 LLM minutes per shipped cycle | wave 174 | 97.9 | - | - | ok | 12 |
| PQ5 LLM minutes per shipped cycle | waves 165-174 | 83.1 | - | - | ok | 68 |
| PQ5 LLM minutes per shipped cycle | waves 145-174 | 109.6 | - | - | ok | 179 |
| PQ5 audit rounds per cycle | wave 174 | 1.381 | - | - | ok | 21 |
| PQ5 audit rounds per cycle | waves 165-174 | 1.389 | - | - | ok | 126 |
| PQ5 audit rounds per cycle | waves 145-174 | 1.739 | - | - | ok | 341 |
| PQ5 fresh tokens per shipped cycle | wave 174 | 1,712,481 | - | - | ok | 12 |
| PQ5 fresh tokens per shipped cycle | waves 165-174 | 1,220,145 | - | - | ok | 68 |
| PQ5 unmeasured calls | waves 165-174 | 10 | - | - | ok | 126 |
| PQ5 unmeasured calls | waves 145-174 | 386 | - | - | ok | 341 |

The boundary report also printed this health line (console boundary log 2026-11-16):

`health: wave 174 · PQ6 13.000/day · top idle cause bench · top fail class capability · PQ7 0.388 · PQ8 0.861 · PQ9 0.343`

Notes:

- The PQ4 system-fail rate of waves 145-174 is now inconclusive at 0.070. Its low bound is 0.048, under the threshold of 0.050, so the breach of the first snapshot is gone.
- Wave 174 had no deadline stop and no system fail.
- PQ5 cost per shipped cycle of wave 174 is 97.9 LLM minutes. That is more than the 83.1 of the 10-wave window.
- Cycle 2405 is the largest single cost: 255.1 LLM minutes to a FAIL verdict, with 208.5 waste minutes (`status-174.json`).

## 3. Code quality (CQ) readings

| Index | Window | Value | 95% interval | Threshold | Status | n |
|---|---|---|---|---|---|---|
| CQ1 | tree | 0.900 | - | 0.878 | ok | 14416 |
| CQ1 | wave 174 (changed units) | 0.808 | 0.720-0.874 | 0.900 | **breach** | 99 |
| CQ2 | wave 174 | 3.0 | 3.0-3.0 | 3.0 | ok | 12 |
| CQ2 | waves 165-174 | 2.0 | 2.0-3.0 | 3.0 | ok | 67 |
| CQ2 | waves 145-174 | 2.0 | 2.0-3.0 | 3.0 | ok | 178 |
| CQ3 interface depth | tree | 14.310 | - | 5.0 | ok (shadow) | 350 |
| CQ4 duplication | tree | 20.492 | - | 45.4 | ok (shadow) | 179389 |
| CQ4 duplication | wave 174, added lines | 8.454 | - | 45.4 | ok (shadow) | 828 |
| CQ5 judgment surface | tree | 57 | - | - | ok (shadow) | 147 |

The boundary report printed `QUALITY_CQ1_BREACH` for wave 174: CQ1 0.808, interval 0.720-0.874, threshold 0.900, n=99 (console boundary log 2026-11-16). The JSON has the same event.

The CQ1 readings of each cycle in the wave 174 window:

| Cycle | CQ1 | n |
|---|---|---|
| 2399 | 0.933 | 15 |
| 2404 | 0.900 | 10 |
| 2406 | 0.750 | 8 |
| 2411 | 0.850 | 20 |
| 2414 | 0.773 | 22 |
| 2418 | 0.750 | 4 |
| 2419 | 0.750 | 4 |
| 2420 | 0.688 | 16 |

Cycles 2407, 2410, 2412 and 2416 changed no Go unit. Each has the warning `QUALITY_VACUOUS_DENOMINATOR`. All four are docs or practice items.

Notes:

- The whole interval of the wave 174 CQ1 reading is under 0.900. Six of the eight cycles with data are under 0.900.
- The CQ4 duplication of added lines went from 0.000 (n=95) at the first snapshot to 8.454 (n=828). Both are far under 45.4.

## 4. Shadow projected stops (evidence for the shadow stage contract)

| Reading | Wave 174 | Waves 165-174 | Waves 145-174 |
|---|---|---|---|
| Spend-limit projected stops | 6 | 34 | 95 |
| Spend-limit stops | 0 | 0 | 0 |
| False stops | 3 | 25 | 61 |
| Handoff projected stops | 0 | 2 | 5 |

- The spend limit stays at shadow, so it stopped no cycle. A projected stop is a stop that the stage records at shadow and does not apply.
- In wave 174, the spend limit made a projected stop on cycles 2401, 2403, 2404, 2405, 2414 and 2418. Cycles 2404, 2414 and 2418 shipped, so they are the 3 false stops.
- The false-stop share of wave 174 is 3 of 6 (0.50). The share of waves 165-174 is 25 of 34 (0.74).
- These counts are the evidence for the F7 retune in `docs/architecture/shadow-stage-contract.md` (PR #1472).

## 5. Lane use, idle causes and time

### 5.1 Lane use

| Wave | Cycles | Wall hours | Used slot hours | Capacity slot hours | Average live lanes | Utilization (PQ7) |
|---|---|---|---|---|---|---|
| 172 | 4 | 7.0 | 16.4 | 55.9 | 2.34 | 0.292 |
| 173 | 21 | 17.7 | 40.7 | 141.3 | 2.30 | 0.288 |
| 174 | 20 | 15.8 | 49.0 | 126.5 | 3.10 | 0.388 |

The rows of waves 172 and 173 come from the 2026-11-15 snapshot (PR #1483). The row of wave 174 comes from `latency-174.json`.

| Measure (`status-174.json`) | Wave 172 | Wave 173 | Wave 174 |
|---|---|---|---|
| Width sized / realized | 8 / 3 | 8 / 2 | 8 / 2 |
| Landed | 4 | 12 | 11 |
| Conflict rate | 0.75 | 0.58 | 0.73 |
| Optimistic admissions / failures | 0 / 0 | 3 / 0 | 8 / 3 |
| LLM minutes per shipped cycle | 83.3 | 78.5 | 103.3 |
| Convergence rate | 1.00 | 0.57 | 0.55 |
| Median LLM minutes to verdict | 93.9 | 37.3 | 42.2 |
| Median fail-fast ratio | - | 0.38 | 0.25 |

### 5.2 Idle causes (PQ8)

The top idle cause of wave 174 is `bench`, with a share of 0.861 (console boundary log 2026-11-16). One cause over 50% is a breach at shadow.

At the boundary, the family `agy-claude` was benched for `usage_probe` from 2026-11-16 07:33 to 2026-11-17 07:33 (+08:00). The evidence text is "CLAUDE AND GPT MODELS week window 100% used, resets 28h 38m", with 393 strikes (`status-174.json`).

| Wave | Top idle cause | Share | Status |
|---|---|---|---|
| 171 | run_cap | 0.880 | breach |
| 172 | operator_stop | 1.000 | breach |
| 173 | family_held | 0.815 | breach |
| 174 | bench | 0.861 | breach |

### 5.3 Where the lane time went

The lane time of wave 174 is 48.7 hours. The gap between phases is 1.0% of it (`latency-174.json`).

| Phase | Hours | Share of lane time | Model hours | Host hours | Gates | Gate hours | Median gate minutes |
|---|---|---|---|---|---|---|---|
| build | 24.0 | 0.492 | 14.0 | 9.9 | 33 | 9.9 | 15.6 |
| ship | 13.3 | 0.273 | 0.0 | 13.3 | 0 | 0.0 | - |
| audit | 8.4 | 0.172 | 1.6 | 6.8 | 28 | 6.7 | 11.3 |
| tdd | 1.2 | 0.024 | 1.1 | 0.0 | 24 | 0.0 | 0.0 |
| scout | 0.9 | 0.018 | 0.8 | 0.0 | 20 | 0.0 | 0.0 |
| triage | 0.4 | 0.008 | 0.4 | 0.0 | 20 | 0.0 | 0.0 |

Five more phases ran once or twice each: architecture-design, test-amplification, memo, fault-localization and spec-verify. Together they hold less than 0.3% of the lane time.

Notes:

- PQ9 gate share is 0.343: the gate total of 16.7 hours divided by the lane time of 48.7 hours. The plan baseline of waves 151 to 160 is 8% to 15%.
- Host time is about 62% of the lane time, and model time is about 37% (sum of the phase rows, synthesis).
- The ship phase has 30 spans for 20 cycles and no model time. Cycle 2419 looped between ship and re-audit for about 3 hours (section 6).
- The `latency-174.json` warning says that 71 phase outcomes had no dispatch signal and the reader skipped them.

### 5.4 Families

| Family | Builds | Passes | Lane hours per pass |
|---|---|---|---|
| agy | 20 | 15 | 3.90 |
| claude | 20 | 14 | 4.15 |
| agy-claude | 1 | 1 | 1.61 |

## 6. Events at the boundary

All facts in this section come from the console boundary log 2026-11-16, except where a file is named.

1. Wave 174 ended with 11 of 20 cycles shipped.
2. Verdicts: 11 PASS, 4 FAIL (2401, 2403, 2405, 2408) and 5 `SKIPPED_UNKNOWN` (`status-174.json`).
3. The top fail class of wave 174 is `capability`.
4. The five `SKIPPED_UNKNOWN` cycles ended on `triage-empty-commitment`, each after about 4 to 5 LLM minutes (`status-174.json`).
5. The wave 175 launch halted once on `LOOP_PIPELINE_BLOCKER_HALT`, rule `no-work-handoffs`.
6. The no-work cycles were 2409, 2413, 2415 and 2417, each about 4 LLM minutes.
7. Two of those items were already shipped on main.
8. The declared files of the other two missed the protected file that the scout planned.
9. Cycle 2419 looped between ship and re-audit for about 3 hours.
10. The cause: tests leaked `.evolve` ledgers under `go/`. PR #1490 fixed it.
11. The wave 174 `run.json` and the wave status disagree on the verdicts of some cycles.
12. Example: cycles 2402 and 2403 are PASS in `run.json`, but `SKIPPED_UNKNOWN` and FAIL in the wave status.

The verdict difference (item 12) and the cycle 2399 difference (section 1) have one effect. Two readers count different ships for one wave: 12 in the quality panel and 11 in the wave status.

## 7. Movement since the 2026-11-15 snapshot

The 2026-11-15 snapshot (PR #1483) was taken at the start of wave 174. This table holds only the measures that both snapshots report.

| Measure | 2026-11-15 | 2026-11-16 | Movement |
|---|---|---|---|
| PQ1, waves 145-174 (n) | 0.522 (322) | 0.525 (341) | up 0.003 |
| PQ1, waves 165-174 (n) | 0.533 (107) | 0.540 (126) | up 0.007 |
| PQ4 deadline, waves 145-174 | 0.084 | 0.079 | down 0.005 |
| PQ4 system-fail, waves 145-174 | 0.075, **breach** | 0.070, inconclusive | breach cleared |
| PQ4 system-fail, waves 165-174 | 0.037, inconclusive | 0.032, inconclusive | down 0.005 |
| PQ5 LLM minutes per ship, waves 145-174 | 110.1 | 109.6 | down 0.5 |
| PQ5 LLM minutes per ship, waves 165-174 | 79.3 | 83.1 | up 3.8 |
| PQ5 audit rounds, waves 145-174 | 1.755 | 1.739 | down 0.016 |
| PQ5 audit rounds, waves 165-174 | 1.374 | 1.389 | up 0.015 |
| Spend-limit projected stops, waves 145-174 | 89 | 95 | up 6 |
| Spend-limit projected stops, waves 165-174 | 28 | 34 | up 6 |
| False stops, waves 145-174 | 58 | 61 | up 3 |
| False stops, waves 165-174 | 22 | 25 | up 3 |
| Handoff projected stops, waves 145-174 | 5 | 5 | none |
| PQ7, wave 174 | 0.625 (start) | 0.388 (end) | down 0.237 |
| PQ7, wave 174 against wave 173 | 0.288 (wave 173) | 0.388 (wave 174) | up 0.100 |
| PQ8 top idle cause | `family_held` (wave 173: 0.815) | `bench` (wave 174: 0.861) | cause changed |
| CQ1 tree | 0.901 (14378) | 0.900 (14416) | down 0.001 |
| CQ2, waves 165-174 | 2.0 (56) | 2.0 (67) | none |
| CQ3 tree | 14.333 | 14.310 | down 0.023 |
| CQ4 tree | 20.493 | 20.492 | none |
| CQ5 tree | 57 | 57 | none |

The 2026-11-15 snapshot had no wave CQ1 reading. The `QUALITY_CQ1_BREACH` of wave 174 is new in this snapshot.

## 8. Questions for a later study

The first five questions come from the 2026-11-15 snapshot (PR #1483). The status column says how each one moved.

| Id | Question | Status at 2026-11-16 |
|---|---|---|
| 1 | Does the PQ4 system-fail rate stay under 0.05 on the 10-wave window as the waves after 174 add data? | Moved. The 10-wave rate went down to 0.032, and wave 174 had 0 system fails. The 30-wave breach is gone. |
| 2 | Does `family_held` stay the top idle cause? | Moved. The top cause of wave 174 is `bench` (0.861), a quota bench of `agy-claude`. |
| 3 | Does the early fast-forward check (PR #1466) remove the ship-retry tail of the kind that cycle 2382 showed? | Open. Cycle 2419 had a 3-hour ship tail, but its cause was a ledger leak (PR #1490). |
| 4 | What is the false-stop share of the spend limit when F7 retunes it from the shipped distribution? | Open. F7 is not retuned yet. The share is 0.50 in wave 174 and 0.74 in waves 165-174. |
| 5 | Do waves that end on one long cycle repeat? | Moved. Wave 174 ended on cycle 2419 (about 3 hours). Waves 173 and 174 both ended on a long cycle. |
| 6 | Does the CQ1 breach of the changed units repeat in wave 175? | New. |
| 7 | Do the quality panel and the wave status agree on the cycles and verdicts of one wave after a fix? | New. Today they count 12 and 11 ships for wave 174. |
| 8 | Does PQ9 stay near 0.34? If yes, the gates, not the models, set the lane time. | New. |
| 9 | Does the scout check the item against main and the declared files before it plans? | New. Four no-work cycles cost about 16 LLM minutes and one launch halt. |
