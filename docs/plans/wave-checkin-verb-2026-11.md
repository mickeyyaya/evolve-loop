# Plan: one verb decides if the running wave is healthy

- **Status:** built, 2026-11-21. Two review rounds ran on the design. The build changed D7 and D8 after the replay (F2.15, F2.16).
- **Design:** this file. **Package note:** [internal-wavehealth.md](../architecture/packages/internal-wavehealth.md) (written in the build step).
- **Evidence:** [the wave 185 incident record](../incidents/2026-11-21-wave-185-verification-lock-stall.md) and [the stall plan](wave-stall-detection-and-verification-capacity-2026-11.md). Both are on another branch at the time of writing.
- **Spec of record:** Part A of the console check-in SOP (`docs/operations/console-checkin-sop.md`, on another branch).

## 1. Request

The operator said on 2026-11-21: "Make sure we have a static checkin SOP that no LLM can make mistakes if they follow this static flow". Then: "Turn those SOPs to executable code that guarantee the flow is healthy or not".

Wave 185 ran 8 lanes for 5.7 hours and shipped 0 of 9 cycles. At about 10:00Z the console read "0 of 8 shipped, load 21" and took no action for about 3.8 hours. The SOP gives the steps. A step that the console reads by hand still depends on the reader. This plan moves the decision into code.

## Terms used throughout

- **Check-in:** one pass over the steps A1 to A10 of Part A of the SOP.
- **Snapshot:** the facts of one check-in, as plain values. It holds no handle to a file or a process.
- **Evaluator:** the pure function from a snapshot to the step results and one outcome.
- **Outcome:** CONTINUE, BRAKE or FIX-LOCAL.
- **Reading:** the value that the evaluator compared. **Threshold:** the value that it compared against.
- **Replay:** a check-in that runs on the recorded files of a past wave, with the clock set to a past time.

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F2.1 | The console read 0 shipped, 8 live and a load of 21 at about 10:00Z, and the wave was a stall. | incident record, Timeline |
| F2.2 | `wave.Cycle` has no failure class. `evolve wave status` prints only the top class. | `go/internal/wave/facts.go:32` |
| F2.3 | The failure class of a finished cycle is `category` in `runs/cycle-N/failure-decision.json`. The fingerprint is `fingerprint` in `failure-digests/cycle-N.json`. `cyclefacts` reads both. | `go/internal/cyclefacts/cyclefacts.go:30,110` |
| F2.4 | Each cycle writes `abnormal-events.jsonl`. Three writers exist: `dispatchevents.Writer`, a map in `phasewatchdog` and a formatted string in `subagent/helpers.go`. No reader exists. | `go/internal/dispatchevents/events.go:59`, `go/internal/phasewatchdog/phasewatchdog.go:270`, `go/internal/subagent/helpers.go:45` |
| F2.5 | Wave 185 had 79 `WARN` events of one kind in 9 cycles. 20 of them came in the first 55 minutes (08:55 to 09:50Z). | `runs/cycle-2582..2590/abnormal-events.jsonl`, counted 2026-11-21 |
| F2.6 | Wave 184 shipped 5 of 8 and was healthy. It had 7 `verify-single-flight-unavailable` events in 4 cycles (2574, 2576, 2577, 2578). The SOP rule of A4 ("3 or more cycles") gives BRAKE on wave 184. | `runs/cycle-2574..2578/abnormal-events.jsonl`, counted 2026-11-21 |
| F2.7 | Wave 184 had 3 failed cycles. Two of them (2577 and 2579) have the class `non-progress` and different fingerprints (`build|gate-block|c245fe6b9e73` and `audit|verdict-fail|5bf4cc13a2ec`). The SOP rule of A3 ("the same class in 2 or more cycles") gives BRAKE on wave 184. | `runs/cycle-2576..2579/failure-decision.json`, `failure-digests/` |
| F2.8 | `LOOP_ORCH_LANE_STALLED` fired for wave 185 at 09:09:43, 09:31:07 and 09:42:30Z. The third line is before 10:00Z. Wave 184 had 2 such lines. | `.evolve/signals.ndjson` |
| F2.9 | `gc.disk_floor_gb` has the default 10. `evolve gc` warns under it. `preflight.min_free_gib` has the default 1.0 and makes `wave next` refuse a launch. | `go/internal/gcpolicy/gcpolicy.go:84`, `go/internal/policy/preflight_config.go:10` |
| F2.10 | The loop breaker `core.EvaluateBlockerBreaker` is a pure rule set over `FailureDigest` values. It has the rules guard-class, unexplained, identical-fingerprint and consecutive. Its ceilings are in `failure_policy.thresholds` (identical fingerprint: 3). It honors the acknowledged fingerprints. | `go/internal/core/blocker_breaker.go:216`, `go/cmd/evolve/cmd_loop_blockerbreaker.go:17` |
| F2.11 | A wave that ships 1 cycle in 5 hours does not match A2 (it needs 0 ships). Wave 184 shipped 5 in 8.7 hours, which is 0.57 ships each hour. | SOP A2 "Limit"; `wave-184.json` |
| F2.12 | `LOOP_ORCH_HOST_PRESSURE` fires only when the cap changes, so the newest signal can be hours old. The loop writes one sample each `sample_s` (default 15) seconds to `.evolve/logs/host-pressure.ndjson`, with `ts`, `load1` and `load_per_core`. The high limit is `fleet.orchestrator.window.adaptive.load_per_core_high` (default 1.5). In wave 185 the largest sample was 6.4 per core. | `go/internal/orchestrator/host_pressure.go:16`, `go/cmd/evolve/cmd_loop_hostpressure.go:60,104`, `go/internal/policy/fleet_adaptive.go:47`, the file, read 2026-11-21 |
| F2.13 | `evolve status` builds the main CI reading with `statusMainCI`. The helper calls the GitHub CLI inside the verb. | `go/cmd/evolve/cmd_status.go:218` |
| F2.14 | The breaker rule `consecutive-failures` (ceiling 3) halts on 3 failed cycles in a row. Wave 184 failed 3 cycles (2576, 2577, 2579). Cycle 2578 shipped between 2577 and 2579, so the failures are not in a row. The fixture of C7 holds the breaker verdict on these digests, and the verdict must be no halt. | `go/internal/core/blocker_breaker.go:190`, `failure-digests/` and `runs/cycle-2576..2579`, read 2026-11-21 |
| F2.15 | Wave 184 had 5 `LOOP_ORCH_LANE_STALLED` lines in 4.3 hours (21:47:07, 21:48:07, 23:54:54, 01:31:26 and 02:05:36Z). The SOP rule "3 or more lines in the wave" gives BRAKE on wave 184. At most 2 of its lines fall in one hour. Wave 185 had 3 lines in 33 minutes (09:09:43 to 09:42:30Z). | `.evolve/signals.ndjson` (pid 26667), read 2026-11-21 |
| F2.16 | The SOP says the idle load of this host is 23.6 on 14 cores, which is 1.7 per core. The adaptive window default `load_per_core_high` is 1.5. The key is a vote to narrow the window. A limit at 1.5 fires on an idle host. The SOP limit of 50 on 14 cores is 3.6 per core. | SOP step A8; `go/internal/policy/fleet_adaptive.go:47` |
| F2.17 | Wave 186 (run 20261121T192937Z) had 3 `LOOP_ORCH_LANE_STALLED` lines. They came at 21:58:29Z (lane `main-ci-red-is-polled-while-the-wave-runs`), 22:13:29Z (`ci-test-integration-treewalk-parity`) and 22:39:23Z (`recycle-x3-archive-leaf-writer`). At 23:39Z each live cycle (2593, 2595, 2597, 2598, 2599) had a signal in the last 10 minutes. 3 of 8 cycles had shipped and none had failed. The rule of D7 gave BRAKE: a false positive. | `.evolve/signals.ndjson`, `.evolve/runs/cycle-N/signals.ndjson`, checked 2026-11-21 |

*Implication of F2.6 and F2.7:* the SOP thresholds for A3 and A4 are not safe as written. A replay of the healthy wave 184 must give CONTINUE. So the evaluator needs two changes. The repeat key is the fingerprint, not the class. The events need a volume rule (D4, D5).

## 3. Goals and non-goals

Goals:

1. One function decides the outcome from a snapshot. No LLM reads a procedure.
2. The function closes the gaps A3, A4 and the ship rate.
3. The function returns BRAKE for wave 185 before 10:00Z and CONTINUE for wave 184.
4. A later in-wave brake and a stall wake call the same function.

Non-goals:

- The verb never engages the brake. The BRAKE procedure of the SOP and the item `in-wave-brake-trips-on-a-shared-fault` do that.
- The verb does not build the in-wave brake or the stall wake. This plan only names where they call the evaluator.
- The verb does not run A7 (usage), A9 (watch processes) and A10 (inbox). Those steps print the result GAP with the command to run by hand.
- The verb does not count console seats and suites (A8). No record of them exists. The row names the gap and the item `console-full-suite-guard-during-a-wave`.

## 4. Candidate designs

- **A. A pure evaluator package and a thin verb.** The package `internal/wavehealth` has `Evaluate(Snapshot) Report`. The verb gathers the snapshot from the existing readers and prints the report.
- **B. The logic inside `wave status`.** Extend `buildWaveStatus` with the steps. The report grows a `checkin` section.
- **C. A rules file and a generic engine.** A JSON file lists the steps as data (reading name, operator, threshold). A small engine runs it over a map of readings.

| Criterion (score 0 to 2) | A | B | C |
|---|---|---|---|
| One home for each rule | 2 | 1 (rules sit beside the status text) | 2 |
| Test without files, clock or processes | 2 | 0 (the builder reads the disk) | 1 |
| A later brake and stall wake call it | 2 | 0 (they cannot import `package main`) | 2 |
| No new language to maintain | 2 | 2 | 0 (an expression engine) |
| Rules with logic (distinct cycles, a rate, a "first reading") | 2 | 2 | 0 (data cannot hold them) |
| Cost to reach 100% coverage of changed code | 2 | 1 | 1 |
| **Total** | **12** | **6** | **6** |

*Why A wins:* the in-wave brake runs inside the loop process. The stall wake runs in `wave watch`. Both need the rule as a library call, and only A gives that. Some steps count sets (the cycles that hold one event kind) and compute a rate. A rules file cannot express them without a new language. B ties them to `package main`.

*What changes our mind:* C becomes the better design if two things hold. The rule set grows past about 20 steps, and operators must edit it without a build. Today there are 8 deciding steps.

## 5. Decisions

| Id | Decision | Because | Rejected |
|---|---|---|---|
| D1 | The package `go/internal/wavehealth` holds `Snapshot`, `Evaluate`, `Report`, `Step` and the outcome type. It reads no file and no clock. | F2.1, section 4 | B and C (section 4) |
| D2 | `Evaluate` returns one `Step` for each of A1 to A10 in order, and one outcome. A8 has three rows: A8-disk, A8-load and A8-seats. A step has `ID`, `Reading`, `Threshold`, `Result` and `Action`. | the SOP table | none |
| D3 | The outcome is BRAKE when any step is BRAKE and the brake is not held. Otherwise FIX-LOCAL when any step is FIX-LOCAL. Otherwise CONTINUE. A held brake turns each BRAKE result into the result HELD. | SOP "outcome rule" | a fourth outcome BRAKE-HELD (the item allows three) |
| D4 | A3 calls `core.EvaluateBlockerBreaker` with the config that the loop builds, and the acknowledged fingerprints. A shared builder `blockerBreakerConfig` in `cmd_loop_blockerbreaker.go` serves the loop and the verb. The verb adds no counter and no ceiling. The builder is a pure mapping. The loop keeps the reconcile writes of the resolved fingerprints, so the verb can lag the loop by one pass on the acknowledged set. A fingerprint in 2 or more waves stays a named GAP. | F2.3, F2.7, F2.10 | a second fingerprint counter with `repeat_ceiling` (a second belief; with the class as key it gives BRAKE on wave 184) |
| D5 | A4 counts the events with severity WARN or higher, for each event type. BRAKE when one type has `abnormal_min_events` or more events **and** `abnormal_min_cycles` or more cycles. | F2.5, F2.6 | 3 cycles alone: it gives BRAKE on wave 184 |
| D6 | A2 has two BRAKE rules. Both need `stall_min_age_minutes` of age and `stall_min_live_lanes` of live lanes. STALL: ships are 0. SLOW: ships each hour are under `min_ships_per_hour`. | F2.11 | a rule with no lane condition: a one-lane wave is slow by design |
| D7 | A5 counts the `LOOP_ORCH_LANE_STALLED` lines from the wave start to `Now`. BRAKE when `stall_lines` lines fall inside `stall_window_minutes` (default 60). The stall limit stays the field `stall_minutes` of each signal. The verb copies no value of `fleet.orchestrator.stall_minutes`. | F2.8, F2.15 | the SOP count over the whole wave (it gives BRAKE on wave 184); a second copy of the key |
| D8 | A8 disk: FIX-LOCAL when free bytes of the project root are under `gcpolicy.Policy.DiskFloorBytes()` (key `gc.disk_floor_gb`, safe for a nil block). A8 load: BRAKE when each host sample of the last `load_sustained_minutes` is at or over `load_per_core_limit` (default 3.5). It needs 2 samples. Fewer give the result GAP. | F2.9, F2.12, F2.16 | `preflight.min_free_gib`: 1.0 weakens the SOP floor of 10. `load_per_core_high` of the adaptive window. It is a narrowing vote, and 1.5 fires on an idle host. The first design used it, and the replay of wave 185 braked at 08:43Z on the start-up load. The newest host pressure signal: not a periodic reading. A state file: the verb must write nothing |
| D9 | The new thresholds live in `policy.json` under `wave.checkin`: `stall_min_age_minutes`, `stall_min_live_lanes`, `min_ships_per_hour`, `abnormal_min_cycles`, `abnormal_min_events`, `stall_lines`, `stall_window_minutes`, `load_per_core_limit` and `load_sustained_minutes`. `Policy.WaveCheckinSettings` resolves them with the disk floor. The failure ceilings stay in `failure_policy`. A value of 0 or less falls back to the compiled default. The loader refuses an unknown key (`strict_keys.go`). | the `wave` block in `policy_wave.go` | a new top-level key |
| D10 | Each reading in the snapshot is optional (`Known`). A reading that was not gathered gives the result GAP. A reading that was gathered and matches no row gives BRAKE with the reading `unmapped reading`. The case today is a main CI state that the table does not list. | SOP rule R2; review finding on future callers | silent OK; BRAKE for a missing reading (the in-wave brake has no CI reading) |
| D11 | `Evaluate` cuts each list at `Now` itself: the events, the stall lines, the host samples and the failure digests. A digest carries the time of its cycle end (the write time of `failure-digest.json`). The gatherers read all lines and stay free of rules, except the lane events of D14. | the proof (section 8); a failed cycle that ends after `Now` must not count | the cut in each gatherer (N copies of one rule) |
| D12 | The verb exit codes: 0 CONTINUE, 3 BRAKE, 4 FIX-LOCAL, 2 a read failure, 10 usage. They live in the verb, not in `wavehealth`. | the item | none |
| D13 | The code that reads `abnormal-events.jsonl` goes into `dispatchevents` (`ReadEvents(workspace)`), the home of the format and of the file name. | one home for each format | a reader in the new package |
| D14 | A stall line counts toward A5 only while its lane is silent at `Now`. The lease made the stall line, so its rule `orchestrator.CycleFreshest` decides what silent means (cycle signals and phase files). A lifecycle signal of the lane (launch, exit, backfill, late exit) is also a lane event. A deadline or stall line is not. Each event is at or before `Now`. `WaveRead.LaneLastEvent` returns the latest time, the gatherer puts it in `StallLine.LastEvent`, and `Evaluate` drops each line with `Recovered()`. | F2.17 | the cycle signals alone (a lane with only stdout is wrongly silent); a longer window (it hides stalls); a check by hand |

*Callers later:* the in-wave brake (item `in-wave-brake-trips-on-a-shared-fault`) builds a `Snapshot` from the pool state in the loop and calls `Evaluate`. The stall wake (item `wave-watch-wakes-on-a-stall`) calls `Evaluate` on each poll of `wave watch`. This plan builds neither. Each reading is optional, so a caller fills only what it has, and the rows without a reading give GAP.

## 6. Components

| Id | Component | Red test first |
|---|---|---|
| C1 | `policy.WaveCheckinPolicy`, `Policy.WaveCheckinSettings` and the unknown-key refusal | `policy_wave_checkin_test.go`: defaults, each key overrides, the reused keys, an unknown key refused |
| C2 | `dispatchevents.ReadEvents` | a table of lines. It has one real line from each writer. It has a corrupt line (an error with the line number). It has an absent file (no error) |
| C2b | `blockerBreakerConfig` shared by the loop and the verb | the loop test of the breaker stays green; a new test checks the mapping of each ceiling |
| C3 | `wavehealth.Evaluate` for A1 to A6 and A8 | one test for each row of the table, and one for each threshold edge |
| C4 | `wavehealth` render of the report (text rows and JSON) | the exact text of one report |
| C5 | the snapshot gatherer in `cmd/evolve/cmd_wave_checkin.go` | the verb on a plane built from files |
| C6 | the verb `wave checkin` in the `wave` handler and its usage text | exit codes through `runWaveWith` |
| C7 | the replay fixtures of waves 185 and 184 | the proof of section 8 |
| C8 | docs: `wave-operations.md` section 2, the package note, the policy table in `policy-config.md` | `evolve docs ste-lint --strict` |

## 7. Order

1. C1, because each other part reads the settings.
2. C2, because the gatherer reads it.
3. C3 and C4, the pure part. Tests need no file.
4. C5 and C6, the verb.
5. C7, the replay. It runs the real verb on the fixtures.
6. C8, the docs, then the seats.

## 8. Verification

The snapshot sources:

| Step | Source | Reader |
|---|---|---|
| A1 | the brake file; the live run leases | `loopchain.BrakeEngaged`, `waveLoopLive` |
| A2 | the wave record (`run_id`, start); the landed cycles; the latest pool signal `fields.live` | `readWaveOf`, `wave.RunFrom`, `wave.PoolLive` |
| A3 | `failure-digest.json` of each cycle of the wave, the acknowledged fingerprints, the lane deferrals, `failure-decision.json` for the class counts | `core.CollectBatchFailureDigests`, `core.LoadResolvedFingerprints`, `core.CollectBatchLaneDeferrals`, `cyclefacts.ReadDecision` |
| A4 | `runs/cycle-N/abnormal-events.jsonl` of each cycle of the wave | `dispatchevents.ReadEvents` |
| A5 | `LOOP_ORCH_LANE_STALLED` in the loop stream | `readWaveOf` (`Loop`) |
| A6 | the main CI run of `required.yml` | `statusMainCI` (a hook in `waveEnv`) |
| A8 | free disk of the project root; the host samples of the last `load_sustained_minutes` | `env.diskFree`, the file `.evolve/logs/host-pressure.ndjson` |
| A7, A9, A10 | none | named GAP rows |

The proof:

| Test | Fixture | Result that must hold |
|---|---|---|
| Wave 185 replay, through `runWaveWith` | the real record, 9 abnormal-event files, the pool and stall signals of pid 13795, the host samples (one each minute) | exit 3. A check each minute gives the first BRAKE at 09:33:37Z, from A4 only (the 15th lock event at 09:33:15Z). Without the host samples the result is the same. At 09:43Z A4 and A5 both brake (the third stall line is at 09:42:30Z). At 10:20Z A2, A4 and A5 brake and the text names STALL and SHARED FAULT |
| Wave 186 replay, through `runWaveWith` | the 3 stall lines and launch lines of pid 51538, the lane scope and 2 real signals of cycles 2592, 2593 and 2595 | A5 is BRAKE at 22:40Z (3 silent lanes) and OK at 23:39Z, with the reading "3 line(s) recovered" |
| Wave 184 replay, through `runWaveWith` | the real record, its abnormal-event files, its 3 failure digests, its ships, its signals | exit 0 at 4 clock times in the wave (23:00, 01:00, 03:00 and 04:30Z). The text ends with `outcome: CONTINUE`. A3 reads the 3 failed cycles, and no breaker rule matches |
| A failed `required.yml` run | a hook value | exit 4; the A6 row names `evolve ci classify` |
| An unlisted CI state | a hook value | exit 3; the reading is `unmapped reading` |
| Each threshold | one test for each key | the outcome changes when the key changes; the value `n-1` gives CONTINUE and `n` gives BRAKE |
| A loop-only snapshot | a snapshot with only the readings the loop has (no CI, no disk) | no BRAKE from a missing reading; the rows are GAP |
| Read-only | a plane tree | the hash of the whole project tree is equal before and after the run |

Each test is red before the code exists. Each changed function must have 100% coverage (`evolve cover changed`).

## 9. Risks

| Risk | Control |
|---|---|
| The defaults of D5 and `stall_lines` come from waves 184 and 185. The replays of those waves cannot prove them. The first replay of wave 184 also found the stall rule of the SOP wrong (F2.15). | The replays prove the wiring. Boundary tests prove each edge. The runs of earlier cycles are gone from the runtime, so no third wave is available. Open question 2 tracks it. |
| The sample file is absent or old. | The result is GAP, never OK, and the row says why. |
| The check-in calls the GitHub CLI through `statusMainCI`. | The hook is an injected function. A failure gives FIX-LOCAL with the action `evolve doctor probe gh`, as the SOP says. |
| The SOP text keeps a copy of the numbers. | The SOP rows of A2 to A5 and A8 must name `evolve wave checkin` and the `wave.checkin` keys, with no number. The numbers live in `policy-config.md`. This plan does not block on it. |

## 10. Open questions

1. The SOP rule "the same fingerprint in 2 or more waves" has no reader today. Name an item for it. `evolve kb graph recurrences` needs the fingerprint first.
2. Are 15 events and 3 cycles the right A4 numbers? Two waves are a small sample. The health readings series can give more waves.
3. The SOP says "load 50 at two check-ins". The verb uses 3.5 per core for 15 minutes, which needs no state. The SOP text must follow.
4. The host samples exist only for the current wave. The load row of an earlier wave is a GAP in a replay.
5. A replay cuts events, stall lines, samples and digests at `Now`. Ship counts stay final. A replay of wave 184 reads 4 of 7 shipped, because cycle 2563 has no landing intent now.
