# Test latency baseline (pre-refactor, 40be980)

- Packages: **107**
- Top-level tests: **3094**
- Aggregate package wall time: **685.4s** (sum of parallel-aware per-package times)
- Fully-serial upper bound (Σ test elapsed): **264.8s**

## Slow packages (> 5.0s wall) — optimization targets

| Package | Wall (s) | Tests | Σserial (s) | Slowest test | Slowest (s) |
|---|--:|--:|--:|---|--:|
| cmd/evolve | 205.37 | 319 | 160.34 | TestE2EPipeline_AuditWarn_FluentShips_StrictBlocks | 78.12 |
| internal/bridge | 47.39 | 317 | 42.90 | TestRealTmux_E2E_LiveInjection_UnblocksAgent | 5.09 |
| internal/phases/ship | 22.81 | 208 | 18.88 | TestNative_Q_PluginVersionBump_RePins | 0.47 |
| internal/core | 13.93 | 197 | 9.73 | TestCycleScenarios | 1.10 |
| internal/rollback | 11.21 | 49 | 7.54 | TestDefaultGhDeleteRelease_ViewSucceeds_DeleteSucceeds | 3.42 |
| internal/releasepreflight | 7.97 | 34 | 3.89 | TestDefaultSimulationRunner | 3.69 |
| internal/releasepipeline | 7.61 | 80 | 3.47 | TestRunMarketplacePollLib_NoEnvVar_NoPanic | 1.26 |
| internal/adapters/observer | 6.78 | 23 | 3.80 | TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows | 1.02 |
| internal/fanoutdispatch | 5.55 | 21 | 3.28 | TestRun_ConsensusCancelTerminatesSurvivors | 2.01 |

## Slowest 25 tests

| Test | Package | Elapsed (s) |
|---|---|--:|
| TestE2EPipeline_AuditWarn_FluentShips_StrictBlocks | cmd/evolve | 78.12 |
| TestE2EPipeline_IntentPhase_RunsAndShips | cmd/evolve | 40.71 |
| TestE2EPipeline_AuditFail_RunsRetro_NoShip | cmd/evolve | 37.48 |
| TestRealTmux_E2E_LiveInjection_UnblocksAgent | internal/bridge | 5.09 |
| TestRealTmux_MultilineSpecialCharPrompt | internal/bridge | 4.86 |
| TestRealTmux_ConcurrentSessionsIsolated | internal/bridge | 4.84 |
| TestRealTmux_NamedSessionResume | internal/bridge | 4.67 |
| TestRealTmux_ArtifactTimeout | internal/bridge | 4.39 |
| TestRealTmux_HappyPath | internal/bridge | 4.21 |
| TestDefaultSimulationRunner | internal/releasepreflight | 3.69 |
| TestDefaultGhDeleteRelease_ViewSucceeds_DeleteSucceeds | internal/rollback | 3.42 |
| TestRealTmux_Interactive_ClaudeSingleSelect_AutoEnter | internal/bridge | 3.09 |
| TestRealTmux_Interactive_CodexTrust_AutoRespondAtBoot | internal/bridge | 2.35 |
| TestRealTmux_Interactive_ClaudeMultiSelect_AutoSubmit | internal/bridge | 2.33 |
| TestRealTmux_Interactive_StuckAutoRespond_TripsLoopGuard | internal/bridge | 2.18 |
| TestRealTmux_Interactive_AgyPermission_AutoYes | internal/bridge | 2.11 |
| TestRun_ConsensusCancelTerminatesSurvivors | internal/fanoutdispatch | 2.01 |
| TestDefaultGhDeleteRelease_ViewSucceeds_DeleteFails | internal/rollback | 1.90 |
| TestRunMarketplacePollLib_NoEnvVar_NoPanic | internal/releasepipeline | 1.26 |
| TestCycleScenarios | internal/core | 1.10 |
| TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows | internal/adapters/observer | 1.02 |
| TestRun_TimeoutKillsWorker | internal/fanoutdispatch | 1.00 |
| TestRealTmux_BootTimeout | internal/bridge | 0.93 |
| TestRealTmux_Interactive_Escalate_FailsFastWithReport | internal/bridge | 0.90 |
| TestRun_NoEnforceMode_NoKillOnStall | internal/phaseobserver | 0.80 |

_21 tests exceed the 1.0s per-test threshold._
