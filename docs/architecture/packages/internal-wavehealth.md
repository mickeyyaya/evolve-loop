# internal/wavehealth

> Plan: [wave-checkin-verb-2026-11](../../plans/wave-checkin-verb-2026-11.md). Operator use: [wave-operations.md](../../operations/wave-operations.md) §2. Policy keys: [policy-config.md](../policy-config.md#wave-health-check-wavecheckin). The verb: `evolve wave checkin`.

## Purpose

`internal/wavehealth` decides if a running wave is healthy. The operator asked on 2026-11-21 for a check-in that no LLM can get wrong. Part A of the console check-in SOP is the spec. This package is the code of that spec.

`Evaluate(Snapshot) Report` is a pure function. It reads no file, no clock and no process. The verb `evolve wave checkin` gathers the snapshot and prints the report. The in-wave brake (item `in-wave-brake-trips-on-a-shared-fault`) and the stall wake (item `wave-watch-wakes-on-a-stall`) must call the same function. This package does not build them.

## Terms

- **Snapshot:** the readings of one check-in, as values. Each reading is `Known[T]`. A caller fills the readings that it has.
- **Result:** OK, BRAKE, FIX-LOCAL, HELD, GAP or SKIP. HELD is a BRAKE while the brake is held. GAP is a reading that was not gathered. SKIP follows a loop that is not running.
- **Outcome:** CONTINUE, BRAKE or FIX-LOCAL. The exit codes are 0, 3 and 4.

## Rules

| Step | Rule | Source of the threshold |
|---|---|---|
| A1 | Loop not running: SKIP the other steps, outcome CONTINUE. Brake held: each BRAKE becomes HELD. | the SOP |
| A2 | Age at or over `stall_min_age_minutes` and live lanes at or over `stall_min_live_lanes`: 0 ships is STALL. Ships each hour under `min_ships_per_hour` is SLOW. Both are BRAKE. | `wave.checkin` |
| A3 | The blocker breaker of the loop (`core.EvaluateBlockerBreaker`) on the failure digests of the wave. A halt is REPEAT, a BRAKE. The reading lists the failure classes. | `failure_policy.thresholds` |
| A4 | One kind of WARN or ERROR event with `abnormal_min_events` events in `abnormal_min_cycles` cycles is a SHARED FAULT, a BRAKE. | `wave.checkin` |
| A5 | `stall_lines` lines of `LOOP_ORCH_LANE_STALLED` inside `stall_window_minutes` is a STALL, a BRAKE. A line counts only while its lane is silent: a line with `LastEvent` after its time is a recovered lane and does not count. | `wave.checkin` |
| A6 | Main CI: completed success and not completed are OK. Completed failure and an unavailable CI are FIX-LOCAL. Any other reading is BRAKE with the reading `unmapped reading`. | the SOP |
| A8-disk | Free disk under the floor is FIX-LOCAL. | `gc.disk_floor_gb` |
| A8-load | Every host sample of `load_sustained_minutes` at or over `load_per_core_limit`, with 2 samples or more, is BRAKE. | `wave.checkin` |
| A7, A8-seats, A9, A10 | GAP rows. The row names the command to run by hand. | none |

`Evaluate` cuts each list at `Now`: the events, the stall lines, the host samples and the failure digests. A replay with a past clock then sees only what the console can see at that time. The ship count and the cycle count are not cut.

One cut sits in the gatherer. The verb cuts the pool signals at `Now` before `wave.PoolLive` reduces them to the live lane count. A caller of `Evaluate` that builds its own snapshot must apply the same cut. A replay test pins it.

One cut sits in the gatherer for A5. `WaveRead.LaneLastEvent` gives `LastEvent` of a stall line: the latest signal of its lane at or before `Now`. The lease made the stall line, so a cycle of the lane counts through `orchestrator.CycleFreshest`, the rule of the lease. A lifecycle signal of the lane counts too, so a lane that exited after its stall is recovered. Wave 186 gave BRAKE at 23:39Z while every live cycle had a signal in the last 10 minutes. This rule removes that false positive.

## Limits

- The exit codes (0, 3 and 4) belong to the verb, in `cmd_wave_checkin.go`. This package has no process vocabulary.
- The outcome is BRAKE when any step gives BRAKE. FIX-LOCAL comes next. The package never engages the brake.
- The defaults of A2, A4 and A5 come from waves 184 and 185. Two waves are a small sample.
- The SOP rules for A3 (same class), A4 (3 cycles) and A5 (3 lines in the wave) give BRAKE on wave 184, which was healthy. This package uses the fingerprint rule of the breaker, a volume rule and a window. The SOP text must follow.
- The load rule reads the file that the loop writes, one sample each 15 seconds while it runs. The verb finds it with `hostSamplesFilePath`, the function that the loop writer uses. It parses the writer struct `hostSampleLine`. The loop can move the file with `--log`, and the verb does not follow that option. A missing file gives GAP.
- The abnormal events are read with `dispatchevents.ReadEvents(workspace)`, which uses the file name `dispatchevents.EventsFile` of the writer. Two other writers (`phasewatchdog` and `subagent`) keep their own copy of the name. A follow-up must move them to the constant.
- A digest gets the write time of its `failure-digest.json` as its end time. A file that cannot be read gets the zero time, and the digest counts as ended.
- The same fingerprint in 2 or more waves is not read. No reader exists for it.
