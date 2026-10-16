# internal/mergegate

## Purpose

`internal/mergegate` is the deterministic kernel of the merge-to-main gate. `DecideCadence` is the advisor: it decides when a completed campaign milestone is promoted. `Promoter` is the actuator: it performs the promotion under the hardened ship path.

## Design

- **The advisor is a pure function.** `DecideCadence` does no I/O. The caller measures every field of `DecisionInput`. The caller also reduces the safety predicates to booleans and counts.
- **`SeverityBlocks` is the comparison of the caller.** It is `router.Severity >= policy threshold`. This package does not copy `router.ParseSeverity`.
- **Cadence order, after the safety floor is clear.** Feature-complete is first: all waves are done and `WavesTotal` is not 0. Then the anti-starvation flush. Then per-wave. Then the batch threshold (waves or churn). Else the advisor keeps accumulating.
- **`WavesTotal` of 0 means unknown.** A non-campaign run has no wave plan. The feature-complete branch must not fire, because 0 >= 0 is true. Per-wave cadence applies.
- **The composition root projects the thresholds from `policy.MergeGateConfig`.** `BatchWaveCount` of 1 or less means per-wave. `CarryoverStallCycles` of 0 disables anti-starvation. `BatchChurnLOC` fires a batch early, so a batch never grows without bound.
- **The promoter is a Humble Object.** `Promoter` owns the stage and verdict state machine. The injected `Executor` holds the dangerous git work. The production `Executor` drives the swarm merge-train and the ship path. The integrity floor, `ship.lock`, the attestation and CI-green all apply unchanged.
- **The package is a leaf.** It imports the standard library and the swarm seams of the promoter. The cadence decision is the most safety-critical judgment in the system. A test must run it without a repo, a campaign or git.

## Invariants

- **Defer wins.** One safety violation forces `CadenceDefer` and `Fire=false`, whatever the progress. The violations are: audit not passed, CI not green, ledger unverified, conflicts present, severity blocks.
- **The check order is fixed.** `safetyDefer` checks the predicates in one order, so the reason is deterministic. The LLM gate phase can only be more conservative than this floor ("model proposes, kernel disposes"). `TestDecideCadence` pins this.
- **Stages `off` and unknown do nothing, not even record.** `shadow` and `advisory` record the planned promotion. They never touch the `Executor`. `promoter_test.go` pins this.
- **Only `enforce` with a `PASS` verdict promotes, exactly once.** A `WARN` or `FAIL` verdict blocks. An unknown stage is a fail-safe no-op.
- **A failed promotion is undone.** `enforce` calls `Rollback` once when `Promote` fails. It then sets `RolledBack`.
- **`RollbackFailed` is never reported with a clean `RolledBack`.** The promotion and its rollback can both fail. Then the target branch can be in an unknown state. The result says so, and the caller escalates instead of trusting main. The double-failure case in `promoter_test.go` pins this.
- **An `enforce` `PASS` with no `Executor` blocks and never panics.** It is the only path that can auto-merge to main. `TestPromoter_EnforcePassNilExecutorBlocks` pins this.
- **The LLM verdict can only withhold promotion.** The `Executor` gates the merge on acceptance. The verdict cannot force an unsafe merge.
