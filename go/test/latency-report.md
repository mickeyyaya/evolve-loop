# Fast suite latency

- Packages: **109**
- Top-level tests: **3070**
- Aggregate package wall time: **473.2s** (sum of parallel-aware per-package times)
- Fully-serial upper bound (Σ test elapsed): **60.0s**

## Slow packages (> 5.0s wall) — optimization targets

| Package | Wall (s) | Tests | Σserial (s) | Slowest test | Slowest (s) |
|---|--:|--:|--:|---|--:|
| internal/phases/ship | 21.88 | 208 | 17.20 | TestNative_Q_PluginVersionBump_RePins | 0.44 |
| internal/core | 12.14 | 197 | 7.66 | TestCycleScenarios | 0.90 |
| internal/rollback | 11.20 | 49 | 7.13 | TestDefaultGhDeleteRelease_ViewSucceeds_DeleteSucceeds | 3.60 |
| internal/releasepreflight | 8.10 | 34 | 4.03 | TestDefaultSimulationRunner | 3.85 |
| internal/adapters/observer | 7.53 | 23 | 3.82 | TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows | 1.04 |
| internal/releasepipeline | 7.31 | 80 | 3.23 | TestRunMarketplacePollLib_NoEnvVar_NoPanic | 0.85 |
| internal/fanoutdispatch | 6.30 | 21 | 3.26 | TestRun_ConsensusCancelTerminatesSurvivors | 2.01 |
| internal/changeloggen | 5.29 | 28 | 0.56 | TestReadGitLog_RealRepo | 0.13 |

## Slowest 25 tests

| Test | Package | Elapsed (s) |
|---|---|--:|
| TestDefaultSimulationRunner | internal/releasepreflight | 3.85 |
| TestDefaultGhDeleteRelease_ViewSucceeds_DeleteSucceeds | internal/rollback | 3.60 |
| TestRun_ConsensusCancelTerminatesSurvivors | internal/fanoutdispatch | 2.01 |
| TestDefaultGhDeleteRelease_ViewSucceeds_DeleteFails | internal/rollback | 1.90 |
| TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows | internal/adapters/observer | 1.04 |
| TestRun_TimeoutKillsWorker | internal/fanoutdispatch | 1.01 |
| TestCycleScenarios | internal/core | 0.90 |
| TestRunMarketplacePollLib_NoEnvVar_NoPanic | internal/releasepipeline | 0.85 |
| TestRun_NilRollbackOverriddenByDefault | internal/releasepipeline | 0.82 |
| TestRun_StallDetectionFires | internal/phaseobserver | 0.80 |
| TestRun_SoftStallNudge_AppendsOnceBelowKillThreshold | internal/phaseobserver | 0.80 |
| TestRun_NoEnforceMode_NoKillOnStall | internal/phaseobserver | 0.80 |
| TestRun_MaxNoProgress_BabbleAgent_Fires | internal/phaseobserver | 0.80 |
| TestWatch_ObserverEventsFileDoesNotMaskStall | internal/adapters/observer | 0.60 |
| TestWatch_StallEmitsIncident | internal/adapters/observer | 0.50 |
| TestWatch_WorkspaceConfiguredButIdle_StillStalls | internal/adapters/observer | 0.50 |
| TestNative_Q_PluginVersionBump_RePins | internal/phases/ship | 0.44 |
| TestRun_MaxNoProgress_ToolUsingAgent_DoesNotFire | internal/phaseobserver | 0.40 |
| TestRun_MaxNoProgress_Disabled_IsLegacyByteIdentical | internal/phaseobserver | 0.40 |
| TestShipFromWorktree_WriteShipBindingWarn_LogsWarn | internal/phases/ship | 0.39 |
| TestShipFromWorktree_CleanWorktreeAheadBranch_Merges | internal/phases/ship | 0.38 |
| TestShipFromWorktree_TreeSHABindingVerifiedLog | internal/phases/ship | 0.38 |
| TestFloorActivationCycle | internal/core | 0.37 |
| TestDefaultRevertAndShip_RevertSucceeds_BinPresent_ShipFails | internal/rollback | 0.37 |
| TestShipFromWorktree_WithAuditBoundTreeSHA_BindingLogged | internal/phases/ship | 0.35 |

_6 tests exceed the 1.0s per-test threshold._
