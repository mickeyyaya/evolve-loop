# Cycle 1853: three defects made a lost tmux server cost 80 minutes (2026-10-09)

The root cause of cycle 1853 is a build agent that ran `tmux kill-server` in its own pane and killed the run's tmux server. That fix has its own record ([cycle-1853-tmux-run-server-kill.md](cycle-1853-tmux-run-server-kill.md)). This record is about three other defects. They did not cause the loss, but they made it slow and they gave it the wrong name.

## What happened

Wave 89, cycle 1853, phase `build`. The times are UTC.

| Time | Event |
|---|---|
| 09:19:20 | The build dispatch starts on `claude-tmux` (session `…-n6-…`). |
| 09:21:08 | The agent kills the tmux server. Every `capture-pane` after this fails with `no server running on /private/tmp/tmux-501/evolve-bridge-p21421`. |
| 09:31:10 | The observer emits `LIVENESS_PHASE_STALLED`: "showed no new transcript for 10m0s (busy)". The stall triage queries the claude usage. The query fails: `recipe: fleet mode: explicit worktree required (refusing process-cwd fallback)`. |
| 09:39:42 | The first review checkpoint (1200 s) extends the wait. |
| 10:00:10 | `BRIDGE_EXIT_ARTIFACT_TIMEOUT`, exit 81, `cause=review_pause`, `waited=2400s`. |
| 10:00:28 | The chain falls back to `agy-claude-tmux`. Its pane shows `⚠ Individual quota reached … Resets in 89h29m48s`. Exit 85. The agy usage query fails with the same fleet refusal. |
| 10:00:44 to 10:42:02 | The phase retry repeats the sequence. The server dies again at 10:02:17 and the stall fires at 10:12:19. Exit 81 comes at 10:41:44, and agy-claude exits 85 at 10:42:02. |
| 10:42:13 | `ORCHESTRATOR_QUOTA_PAUSED`. The checkpoint has reason `quota-likely`, `quotaResetSource` `default` and `quotaResetAt` `2026-10-10T00:07:14+0800`. |

## Root causes

### Defect 1: a lost pane read as "busy" until the 20-minute review

The wait loop (`go/internal/bridge/driver_tmux_wait.go`, `replWaiter.wait`) captured the pane every 2 s. A failed capture only set `captureOK=false`. Nothing asked tmux if the session still existed. The loop waited for the review checkpoint (`state.intervalS`, 1200 s). The stall reviewer paused it after a second interval.

The observer read the last pane-watch snapshot. The bridge process still owned that snapshot, so it stayed `busy=true`.

### Defect 2a: a walk that met a wall was a quota pause, also when another rung failed for a different reason

`bridgechain.WallKeeper` (`go/internal/bridgechain/wall_keeper.go`) read a walk as walled when one rung exited 85. The claude rung failed because its pane was lost. Its usage evidence was `unavailable`. The agy-claude rung met a real wall: the agy-owned Claude quota. The walk surfaced exit 85, and `core.isQuotaWall` paused the cycle.

The claude family had capacity. The brief gives claude at 93 % of its week, and the agy Gemini group at 30 % (5 h) and 78 % (week).

### Defect 2b: the pause wake time was a fixed default

`quotareset.Compute` used a built-in 5.4167 h when no hint file existed. No production code writes `quota-reset-hint.txt`, so every pause got `source=default`. The bench store had the wall's own reset (89 h), but the checkpoint did not read it.

### Defect 3: the usage query had no working directory in fleet mode

`bridge.ControllerFactory.For` built a `Config` with no `Worktree`. Under `EVOLVE_FLEET=1`, `newRecipeDriver` (`go/internal/bridge/recipe_adapter.go`) refuses a launch with no worktree (`errWorktreeRequired`). Every usage query of a fleet lane failed. Thus every stall had `unavailable` evidence.

## The fix

### Defect 1: pane loss

The change is in `go/internal/bridge/driver_tmux_wait_paneloss.go`. After a failed capture, the wait loop runs `tmux has-session` for the session. When the session is gone on 2 consecutive ticks (`paneLossConfirmTicks`), the wait ends. The diagnostic gives the new cause `pane_lost` (`launchoutcome.TimeoutPaneLost`). The classified outcome is exit 81 with `cause_code=pane_lost`.

A healthy capture runs no extra tmux command. A capture failure on a live session does not end the wait. A cancel that comes during the confirmation ends as `context_cancelled`, never as `pane_lost`.

The seal of the cycle names the cause. `epilogueCauseSuffix` writes `teardown=pane_lost: …` into the termination reason. Thus the fingerprint of a lost pane never merges with the fingerprint of an ordinary `review_pause` timeout.

### Defect 2a: the walk rule

A pause needs an exit 85 in the walk. Each other rung that started work must be proven exhausted or be neutral.

- **Proven exhausted.** A usage query read `exhausted`. The `usageevidence.Wrap` decorator writes this into the new field `core.BridgeResponse.UsageExhausted`.
- **Neutral.** The rung never started work: exit 80 (the REPL did not boot), exit 87 (the agy-claude model label is missing) or exit 127 (no binary). A candidate that the walk did not launch is also neutral.
- **Unproven.** The rung started work and failed with no proof, for example an exit 81 stall or a `pane_lost`. Then `WallKeeper` surfaces the failure of that rung, and it names every rung.

The cycle 1853 walk now surfaces exit 81 `pane_lost`, not exit 85. A walk of agy-claude exit 87 then claude exit 85 still pauses. A walk with no exit 85 never pauses, also when a usage read says `exhausted`.

### Defect 2b: the wake time

The change is in `quotareset.Compute` and `checkpoint.withQuotaReset`. The runner wraps a failed walk in `core.WalkError`, and `pauseForQuota` copies its CLIs to `CycleState.QuotaWalkCLIs`. The order of the sources is:

1. the operator override;
2. the hint file;
3. the earliest active quota bench of a walked family (`source=bench`);
4. the usage query of the walked families (`source=usage`);
5. the configured `quota_reset.default_hours`;
6. `source=unknown`.

A bench counts only when its reason is a quota wall (`clihealth.QuotaPattern`) and it is still active. A credential bench and a boot-timeout strike do not count.

An unknown reset is not auto-resumable. The checkpoint has no wake time, `autoResumeMaxAttempts` is 0, and `operatorAction` tells the operator to resume by hand. The `QUOTA-PAUSE` line then says `auto-resume=off`. A pause with a known wake time says `resume-in=Ns`, with a floor of 60 s and a cap of 3600 s.

### Defect 3: the usage query cwd

Each control event gets the owned scratch directory `<workspace>/<family>/bridge-scratch-cwd` (`cliController.perFamilyConfig`). This is `applyScratchCwd`, the probe-launch rule. Thus the fleet guard has an explicit worktree.

## Tests

| Defect | Test |
|---|---|
| 1 | `bridge_test.TestRealTmux_AKilledServerEndsTheWatchingDispatchAsPaneLostWithinOneLivenessInterval` (integration; real tmux on its own socket, the server is killed during the wait); `TestRunTmuxREPL_ATmuxServerKilledMidPhaseEndsTheDispatchAsPaneLostWithinOneLivenessInterval`; `TestRunTmuxREPL_ACaptureFailureOnALiveSessionDoesNotEndTheDispatch`; `TestRunTmuxREPL_OneTickWithoutTheSessionIsNotYetAPaneLoss`; `TestRunTmuxREPL_AHealthyCaptureNeverQueriesTheSession`; `TestRunTmuxREPL_ACancelDuringTheLossConfirmationEndsAsContextCancelledNeverPaneLost`; `TestEpilogueCauseSuffix_APaneLossNeverMergesWithAReviewPauseTimeout` |
| 2a | `TestWallKeeper_AStallWithNoExhaustionEvidenceBeforeTheWallIsNotAQuotaPause`; `TestWallKeeper_AStallWithExhaustionEvidenceBeforeTheWallIsStillAQuotaPause`; `TestWallKeeper_ARungThatNeverStartedWorkIsNeutralSoTheRealWallStillPauses`; `TestWallKeeper_ANonStartAfterTheWallStillSurfacesTheWall`; `TestWalking_ALoneExhaustedUsageReadWithNoWallIsNoPause`; `TestWalking_TheCycle1853WalkWithAnUnprovenPaneLossSurfacesThePaneLoss`; `TestRun_TheCycle1853WalkIsAQuotaPauseOnlyWithExhaustionEvidenceForThePaneLoss`; `TestBridge_OnlyAnExhaustedUsageVerdictMarksTheAttemptAsProvenExhausted` |
| 2b | `TestCompute_AnUnknownResetIsNowAndSaysUnknownNeverAFarFutureDefault`; `TestCompute_TheUsageQueryResetComesAfterTheBenchAndBeforeAConfiguredDefault`; `TestQuotaBoundaryCheckpointer_AnUnknownResetIsNotAutoResumableAndNamesTheOperatorAction`; `TestQuotaBoundaryCheckpointer_ABenchThatIsNoQuotaResetOfTheWalkIsNoEvidence`; `TestQuotaBoundaryCheckpointer_TheUsageQuerySuppliesTheResetOfTheWalkedFamilies`; `TestQuotaBoundaryCheckpointer_TheEarliestActiveBenchSetsTheWakeAt`; `TestRunCycle_AWalledPauseHandsTheWalkedCLIsToTheCheckpoint`; `TestQuotaPause_TheResumeDelayHasA60SecondFloorAndAOneHourCapForEverySourceThatAutoResumes`; `TestQuotaPause_ANonResumablePauseIsNeverScheduled`; `TestCheckpointUsageReset_IsWiredToTheProductionUsageQuery` |
| 3 | `TestControllerFactory_AFleetModeUsageQueryRunsInAnOwnedScratchDirAndReadsTheFakeCLIScreen` |

## Changed decisions

- **T2 in [logic-first delivery](../architecture/logic-first-delivery-design.md) and point 4 of [ADR-0104](../architecture/adr/0104-fallback-is-a-property-of-the-bridge-handle.md).** They said that an exhausted walk that met a wall anywhere is a capacity outcome. Now a pause needs an exit 85, and each other started rung must be proven exhausted or neutral. A stall after a wall with no proof is a FAIL. The console accepted this class on 2026-10-09.
- **The quota wake time.** The old contract said the wake time must always be in the future. Now an unknown reset has no wake time and no auto-resume. The operator resumes by hand.

## Decisions on the open items (console, 2026-10-09)

- **No fresh-session rung for `pane_lost`.** The phase retry already starts a fresh session.
- **No hard-coded `infra-systemic`.** After the tmux isolation fix ([cycle-1853-tmux-run-server-kill.md](cycle-1853-tmux-run-server-kill.md)), a lost pane is a fault in one lane. The `infra-systemic` class halts the fleet, so it is the wrong class. A repeat is caught by the ADR-0072 fingerprint ceiling and by the infra-transient streak (`failureadapter.go`, rule 5).
- **Still open.** Nothing increments `autoResumeAttempts` yet. The resume delay refuses a pause at its cap, but the counter stays at 0 until the resume path counts it.
