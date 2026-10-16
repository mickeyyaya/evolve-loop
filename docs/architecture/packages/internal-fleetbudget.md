# internal/fleetbudget

## Purpose

`internal/fleetbudget` is the pure allocator of the quota-driven fleet budget. It takes four inputs. The first is the measured quota of each CLI family (`quotastate.QuotaState`). The second is the measured pace of the pipeline (`budgethistory.Throughput`). The third is the operator lane envelope and budget tunables (`Config`). The fourth is the current time.

It decides how many lanes the wave runs and how to pace them, and it records why (`DerivedFrom` and `Reason`).

## Design

- **Measurement replaces the static `min_lanes` assertion (#303).** The wave is sized against the real headroom that remains until the reset. The units are the native units of each CLI: the remaining fraction and the reset time. It never uses dollars: subscription CLIs report $0, which is why the old dollar budget was removed. `min_lanes` is now `Config.Floor`. It is the lower bound that the plan never goes below, and the fallback when every signal is unknown.
- **Shadow-safe by default.** The budget branch runs only if the operator gave both tunables: `CapacityCycles > 0` and `0 < Safety <= 1` (`hasBudgetTunables`). Without them, which is the default until an operator opts in through the `fleet.budget` policy block, `Plan` never sizes below `Config.Count`. Wiring it in shadow mode is then byte-identical to the old behavior.
- **Pure and a leaf.** It does no I/O, and the caller injects `now`. It depends only on `quotastate`, `budgethistory` and the standard library. The wiring layer maps `policy.FleetConfig` and the `fleet.budget` block into `Config`, so the allocator stays free of the policy package. `Plan` clamps `Floor >= 1` and `Count >= Floor` against odd operator input.
- **Three decisions, each with a `DerivedFrom` value.**
  - `FromBudget`: the remaining fraction, a reset horizon, the measured pace and both tunables are known. The plan sizes from them.
  - `FromResetPace`: a healthy family reports a reset, but an input for the sizing is missing (no tunables, no pace, or the reset has passed). The plan runs at full width and carries the reset for the operator. A throttle curve is a possible refinement. There is no invented delay now.
  - `FromFloor`: no usable quota signal (an unknown source, or only exhausted families). The plan runs the configured width. Exhaustion is not handled here. The bench-shrink step composes with it downstream.
- **The binding window is the tightest healthy one.** `tightestHealthy` skips exhausted families and families whose source is not `SourceProbed`. Across all remaining buckets it takes the lowest remaining fraction. A weekly cap with 34 percent left outweighs a roomier session window, and the tightest family outweighs a roomier one.
- **The sizing formula.** The budget in cycles is `remaining * CapacityCycles`. The affordable lanes are `budget * Safety / (CyclesPerHour * hoursToReset)`. The lanes times the pace per lane times the hours must stay within the budget times the safety.
- `CyclesPerHour` is the rate of one lane, not of the fleet. The value is capped at `Count` before the integer cast, so a very large `CapacityCycles` cannot overflow `int`. The uncapped value stays for the pacing test.
- **Pace the surplus the floor forces.** When the affordable lanes are fewer than the floor, the plan runs the floor. It adds an idle gap to each cycle. The gap is `dutyGap = Floor / affordable - 1` times the median cycle duration. The effective rate then matches the affordable duty cycle. The delay never exceeds the time to the reset, because the budget refreshes at the reset and the constraint ends.
- **`resetPhrase`** prints `reset has passed` and never a negative duration.
- **`ShadowJoin` is observation only.** It pairs the binding window with the median tokens per cycle (token telemetry, S8). It is the evidence that a later admission control needs. Nothing in `Plan` reads it, and `QuotaJoin` is deliberately not a field of `BudgetPlan`, so it cannot affect a decision. Callers log it and must not feed it to `Plan`.
- It returns `ok == false` when either half is missing, because half a join is not a join. It must not report the minimum seed of `quotastate` as 100 percent remaining, and it must not join against an unknown token count.

## Invariants

- **No budget config, no restriction.** `TestPlan_ShadowByteIdenticalWithoutBudgetConfig` and `TestPlan_ResetPaceWhenNoBudgetConfig` pin that `Plan` returns `Count` however tight the quota is, when the tunables are unset.
- **The tightest window binds.** `TestPlan_TightestWindowBindsAcrossFamilies` pins it (claude with 34 percent against codex with 90 percent).
- **The pacing cap.** `TestPlan_PaceDelayCappedAtResetHorizon` pins that `PaceDelay` is capped at the time to the reset. `TestPlan_HugeCapacityDoesNotOverflow` pins the cap before the integer cast, and `TestPlan_ResetPassedShowsNoNegativeDuration` pins `resetPhrase`.
- **Join and decision are separate.** `TestPlan_DecisionUnchangedByShadowJoin` pins that `Plan` gives the same output with the join present. `TestShadowJoin_NoQuotaSignalYieldsNoJoin`, `TestShadowJoin_NoTokenEvidenceYieldsNoJoin` and `TestQuotaJoin_ZeroValueIsNotAJoin` pin the three absent-evidence cases.

## Findings

- None.
