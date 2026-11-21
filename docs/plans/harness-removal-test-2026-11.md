# Plan: measure which harness components are load-bearing after the M2 baseline

- **Status:** partly built, 2026-11-20 (the test design is built; the removal test has not run, blocked on M2). Earlier status: proposed, 2026-11-20.
- **Parent plan:** [harness-practices-2026-11](harness-practices-2026-11.md) (D11, D12, R9).
- **Decision:** [ADR-0159](../architecture/adr/0159-the-loop-keeps-its-integration-train-and-takes-six-practices-from-the-2026-harness-projects.md) and [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).
- **Research:** [harness-practices-2026-11](../research/harness-practices-2026-11.md) (F10.1, F13.1).

> **Context:** [harness-practices-2026-11](../research/harness-practices-2026-11.md) F13.1 cites an Anthropic result where a smaller harness worked and cost less. Each harness component assumes a model limit that can go stale. This plan specifies a single-component removal test after the M2 baseline of [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

## 1. Precondition

The removal test must wait for the M2 baseline of [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). M2 is `evolve prompt stats`, open as `model-capability-practice-m2-prompt-stats` ([model-capability-practice-2026-10](model-capability-practice-2026-10.md) §7).

**F1.1** [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md) records the M2 prompt size and cycle baselines before any prompt or harness cut.
*Implication:* without M2, a removal test has no baseline to compare ship rate and cost.

The test must not start until M2 merges and records at least 10 cycles of baseline data. No production code or policy change ships under this item before M2 lands.

## 2. Component

The first removal test disables one component: `plan-review`. `plan-review` is an optional evaluation phase under `workflow.phase_enables` (`go/internal/policy/workflow.go:18`).

| Attribute | Value |
|---|---|
| Component | `plan-review` |
| Control point | `workflow.phase_enables` in `.evolve/policy.json` |
| Kernel guards kept | `evolve guard phase`, `evolve guard role`, `evolve guard ship` |
| Change in test arm | Set `"plan-review": "off"` for the test window |

**F2.1** [model-capability-practice-2026-10](model-capability-practice-2026-10.md) §3 forbids the removal of any kernel guard.
*Implication:* the removal test targets optional LLM phases first, never deterministic kernel guards.

Later removal tests can target other optional components one at a time, after `plan-review` completes its window. Never disable two components in one window.

## 3. Window

The removal test runs for a window of 2 waves, with at least 10 cycles in the arm. This matches the 10-cycle window of [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md) and the two-wave evidence rule of [ADR-0159](../architecture/adr/0159-the-loop-keeps-its-integration-train-and-takes-six-practices-from-the-2026-harness-projects.md).

1. Run wave 1 with `plan-review` set to `"off"` in `.evolve/policy.json`.
2. Check the stop rule at the wave 1 boundary before wave 2 starts.
3. Run wave 2 under the same setting if wave 1 passes the stop rule.
4. Compare the 2-wave arm against the M2 baseline at the wave 2 boundary.

## 4. Measure

Every removal test records two primary measures: ship rate and cost of each shipped cycle.

| Measure | Formula | Source |
|---|---|---|
| Ship rate | Shipped cycles divided by total cycles in the window | `evolve wave status` |
| Cost of each shipped cycle | Total USD cost in the window divided by shipped cycles | Cycle telemetry and dossiers |
| Audit red count | Share of cycles that fail `evolve acs suite` after build | `acs-verdict.json` |

**F4.1** [harness-practices-2026-11](../research/harness-practices-2026-11.md) F10.1 shows that token cuts can raise cost for each solved task.
*Implication:* the test must judge cost of each shipped cycle, not raw tokens per cycle.

## 5. Stop rule

Stop the removal test immediately if either condition holds at a wave boundary or mid-wave check:

1. Ship rate falls by 10 percentage points or more against the M2 baseline over 1 wave.
2. Cost of each shipped cycle rises by 15% or more against the M2 baseline over 1 wave.
3. Two consecutive cycles fail audit for scope defects that `plan-review` catches.

When a stop rule fires, restore `"plan-review"` in `.evolve/policy.json` at once. Record the component as load-bearing in `docs/plans/harness-practices-2026-11.md`. If neither stop rule fires across 2 waves, keep `"plan-review"` off and record the saving.

## 6. Candidate designs

Each criterion scores 1 (poor) to 5 (good).

| Criterion | A. Wait for M2, 1 component, 2 waves | B. Remove 3 components at once | C. Run without M2 baseline |
|---|---:|---:|---:|
| Attributes cause to one component | 5 | 1 | 4 |
| Compares against measured baseline | 5 | 5 | 1 |
| Respects ADR-0133 order | 5 | 4 | 1 |
| Total (of 15) | 15 | 10 | 6 |

**A wins** (15 of 15). Option B cannot tell which of the three components caused a drop in ship rate. Option C violates [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md) and lacks a baseline for ship rate and cost.

## 7. Worked example

Suppose the M2 baseline over 10 cycles has a ship rate of 70% and a cost of $4.00 per shipped cycle.

| Wave | Cycles | Shipped | Ship rate | Cost per shipped cycle | Result |
|---|---:|---:|---:|---:|---|
| M2 baseline | 10 | 7 | 70% | $4.00 | Baseline |
| Test wave 1 | 8 | 6 | 75% | $3.60 | Pass; continue to wave 2 |
| Test wave 2 | 8 | 6 | 75% | $3.55 | Pass; `plan-review` stays off |

In a failure case, wave 1 ships 4 of 8 cycles (50%) at $4.90 per shipped cycle. Ship rate fell 20 points, which hits the 10-point stop rule. The operator stops the test and restores `plan-review`.

## 8. Limits and open questions

- **What this design does not do:** this design does not change `.evolve/policy.json` or any Go code today.
- **What a test cannot prove:** a syntax check of this document proves its required sections and STE compliance, not the wave outcome.
- **Open question:** which optional component follows `plan-review` after M2? The operator picks the next candidate from `workflow.phase_enables` after the first 2-wave result.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| Plan document and design check | done | cycle 2549, [`70e796b`](https://github.com/mickeyyaya/evolve-loop/commit/70e796b37c134e95d565d28b3fb10af276188d11) (item `harness-removal-test`, consumed) | none |
| Precondition: M2 baseline `evolve prompt stats` | not started | no `prompt stats` verb on main; the item `model-capability-practice-m2-prompt-stats` is neither open nor consumed | none |
| Removal test: `plan-review` off for 2 waves | not started | none (blocked on M2) | none |
| Later removal tests of other components | not started | none | none |

Only the design is done. The test needs the M2 baseline, and M2 has no verb on main and no inbox item. The test run has no open inbox item either. A follow-up draft is in `followups/harness-removal-test-run-plan-review.json`. The draft names M2 as its blocker.
