# Plan: the fleet admission window follows the load, the memory and the swap of the host

- **Status:** built, 2026-11-21 (6 of 6 components on main at stage `shadow`; rollout steps 4 and 5 not done). Earlier status: proposed, 2026-11-20. Built at stage `shadow` by default. The reviewed design is in `.evolve/evidence/design.md` of the build worktree.
- **Related plans:** [wave-orchestrator-2026-10.md](wave-orchestrator-2026-10.md), [pool-resupply-2026-10.md](pool-resupply-2026-10.md).
- **Package notes:** [internal-hostpressure](../architecture/packages/internal-hostpressure.md), [internal-orchestrator](../architecture/packages/internal-orchestrator.md), [internal-policy](../architecture/packages/internal-policy.md), [cmd-evolve](../architecture/packages/cmd-evolve.md), [cmd-hostsampler](../architecture/packages/cmd-hostsampler.md).

**Terms used throughout:**

- **Sized width:** the smaller of `fleet.count` and `fleet.concurrency`. It is the ceiling of the pool.
- **Host cap:** the number of lanes that the host load allows. It is never above the sized width.
- **Pressure score:** the highest of three ratios. The ratios are the load for each core, the memory floor over the available memory, and the used swap over the swap limit. A score above 1 means pressure.

## 1. Request

The operator wants a fleet of 12 lanes on a 14-core host. A raw count of 12 lanes gives no protection when the host is busy. An earlier run had a load of 50 to 75 at 4 lanes. The cause was composed-tree `make` gates that no lane budget limited.

## 2. Facts

| Fact | Source |
|---|---|
| The only adaptive input of the window today is a landing event: +1 for a clean landing, half for a conflict. | `internal/orchestrator/window.go` |
| No code reads the CPU, the memory or the swap of the host for admission. | `internal/orchestrator` |
| A smaller window never touches a running cycle. It only stops the backfill. | `fleet/pool.go`, `width` and `fill` |
| The idle load of this host is 23.6 on 14 cores, so a fixed load threshold fails at idle. | design §1 |
| `kern.memorystatus_level` gives the free memory in percent on this host (`93` on 2026-11-20). | `sysctl -n kern.memorystatus_level` |
| The `orchestrator` package forbids a ticker, a poll and a sleep (`guard_test.go`). | `internal/orchestrator/guard_test.go` |

## 3. Candidate designs

| | A. Load per core, two bands | B. AIMD on a pressure score | C. CPU tokens for each lane |
|---|---|---|---|
| Inputs | load | load, memory and swap | the CPU history of each lane, and load |
| New code | small | medium | large |
| Sees memory and swap | no | yes | no |
| Match with the existing window | partly | yes: +1 and halve | no |
| Main risk | the load lags, and the idle baseline misfires | one noisy ratio dominates | macOS has no CPU figure for each lane |

## 4. Decision

Build design B. The reasons:

1. It is the only design that sees memory and swap. A lane that fills the memory hurts as much as a lane that fills the CPU.
2. It has the same shape as the window: a slow rise and a fast fall. An operator who knows the window knows the controller.
3. A slow rise and a longer cooldown after a fall prevent a sawtooth.

The controller acts on admission only. It never signals, pauses or stops a running cycle.

**Console review changes to the design.**

- The width of `fleet.RunPool` stays as it is. The pool starts a lane only for an item that it holds, so a width above the supply adds no load.
- The change does not edit `.evolve/policy.json`. The key `fleet.orchestrator.window.adaptive` is absent by default, and absent means that the cap equals the sized width.
- The reader prefers `kern.memorystatus_level` on macOS and falls back to `vm_stat`.
- The parsers of `cmd/hostsampler` move to `internal/hostpressure`. The sampler calls them.
- The sampling goroutine lives in `cmd/evolve`, bound to the run context. The `orchestrator` package gets no timer.

## 5. Components

| Id | Component | Where |
|---|---|---|
| H1 | The parsers, the reader and the pure decision | `internal/hostpressure` |
| H2 | The policy block and its validation | `internal/policy/fleet_adaptive.go` |
| H3 | `Window.SetCeiling`, `HostCapped` and `Intake.Kick` | `internal/orchestrator/window.go` |
| H4 | The idle cause `host_pressure` and the underfill skip | `internal/orchestrator/idle_cause.go`, `pool.go` |
| H5 | The codes `LOOP_ORCH_HOST_PRESSURE` and `LOOP_HOST_PRESSURE_BLIND` | `internal/orchestrator` |
| H6 | The sampling goroutine and `FleetConfig.SizedWidth` | `cmd/evolve/cmd_loop_hostpressure.go`, `internal/policy/fleet.go` |

## 6. Rollout

1. Merge the controller. With the key absent, nothing changes.
2. The console adds `window.adaptive` with `stage` `shadow`. The controller then signals each change of the cap with `applied` false and changes nothing. It also appends each reading to `host-pressure.ndjson` in the log directory of the loop.
3. Read two waves of `host-pressure.ndjson`. Set `load_per_core_high` and `load_per_core_low` from the recorded load of the host with lanes live. Do not guess them.
4. Flip `stage` to `enforce` on a verified record.
5. Then set `fleet.count` to 12, `fleet.min_lanes` to 4 and `window.start` to 8. The controller ramps the width to 12.

## 7. Verification

| Claim | Test |
|---|---|
| A high load halves the cap and keeps the running lanes. | `TestPool_HighPressureCeilingStopsBackfillButKeepsRunningCycles` |
| A low load raises the cap one step for each increase cooldown. | `TestDecide_LowPressureRaisesOneStepPerIncreaseCooldown` |
| Low memory and high swap shrink the cap. | `TestDecide_MemoryBelowMinShrinksEvenWhenLoadIsLow`, `TestDecide_SwapAboveMaxShrinks` |
| A failed read holds the cap and warns once. | `TestDecide_FailedReadHoldsTheCapAndBlindFiresOnceAtTheStreak` |
| 8 admissible items under a ceiling of 12 open 8 lanes and raise no underfill WARN. | `TestPool_SupplyEightAdmissibleOfTwelveCapOpensEightLanesAndRaisesNoUnderfillWarn` |
| Shadow signals only a change and does not cap. | `TestShadowStageDoesNotCap_ButSignals`, `TestHostSampler_ShadowSignalsOnlyOnACapChange` |
| Every reading goes to the samples file, and a write error warns once. | `TestHostSampler_AppendsEveryReadingToTheSamplesFile`, `TestHostSampler_ASamplesWriteErrorWarnsOnce` |
| The key is strict and validated, and absent means cap equals sized. | `TestParse_FleetAdaptiveStrictAndValidated`, `TestFleetConfig_AdaptiveAbsentMeansCapEqualsSized` |
| A host raise does not undo a conflict back-off, and a host apply keeps the quota limit. | `TestWindow_AHostRaiseDoesNotUndoTheConflictBackOff`, `TestStartHostPressure_AnAppliedCapLeavesTheOversightQuotaLimitAlone` |
| The sized width has one home. | `TestFleetConfig_SizedWidthIsTheSmallerOfCountAndConcurrencyAtLeastOne`, `TestHostSized_AllThreeSitesUseTheOneSizedWidth` |

## 8. Risks

- The idle load of 23.6 makes the default thresholds clamp the cap at idle. The shadow stage exists to find this before it costs a lane.
- The window and the controller are two states. The window admits the lower of its landing size and the host ceiling, so neither can lift the other.
- A reader that fails for a long time leaves the cap where it is. The blind code names the fault.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| H1 parsers, reader, pure decision | done | [#1598](https://github.com/mickeyyaya/evolve-loop/pull/1598), [`d4b2f26`](https://github.com/mickeyyaya/evolve-loop/commit/d4b2f264a27b9ad42b186491edaa60b7092ba92e); `go/internal/hostpressure/decide.go` | none |
| H2 policy block and validation | done | [#1598](https://github.com/mickeyyaya/evolve-loop/pull/1598); `go/internal/policy/fleet_adaptive.go` | none |
| H3 `SetCeiling`, `HostCapped`, `Kick` | done | [#1598](https://github.com/mickeyyaya/evolve-loop/pull/1598); `go/internal/orchestrator/window.go:54` | none |
| H4 idle cause and underfill skip | done | [#1598](https://github.com/mickeyyaya/evolve-loop/pull/1598); `go/internal/orchestrator/host_pressure_test.go` | none |
| H5 signal codes | done | [#1598](https://github.com/mickeyyaya/evolve-loop/pull/1598); `docs/architecture/signal-codes.md` | none |
| H6 sampling goroutine, `SizedWidth` | done | [#1598](https://github.com/mickeyyaya/evolve-loop/pull/1598); `go/cmd/evolve/cmd_loop_hostpressure.go`, `go/internal/policy/fleet.go:164` | none |
| Rollout step 2: policy at stage `shadow` | done | [#1606](https://github.com/mickeyyaya/evolve-loop/pull/1606), [`1214752`](https://github.com/mickeyyaya/evolve-loop/commit/1214752b2); `.evolve/policy.json:140` | none |
| Rollout steps 3 to 5: set thresholds, flip to `enforce`, set `fleet.count` to 12 | not started | `.evolve/policy.json` still has `fleet.count` 8 and `min_lanes` 8 | none |

All six code components are on main. The controller runs in `shadow`. Steps 3 to 5 need two waves of `host-pressure.ndjson`, so they have not started. No open inbox item tracks them. A follow-up draft is in `followups/fleet-host-adaptive-enforce-and-12-lanes.json`.
