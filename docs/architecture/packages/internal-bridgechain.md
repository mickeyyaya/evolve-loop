# internal/bridgechain

> Related: [the retro timeout incident](../../incidents/2026-09-14-retro-timeout-sealed-the-cycle.md) and [internal-core-advisor.md](internal-core-advisor.md). This page keeps the package-level detail that the code cannot say.

## Purpose

`internal/bridgechain` makes the CLI and tier fallback chain a property of the bridge handle and not of one caller.

## Design

- **The gap.** The shared phase runner has walked a chain since WS-G1 (`llmroute.Dispatch`) and WS-876 (`DispatchTiered`). On a trigger exit it advances to the next CLI: 80 (REPL boot timeout), 81 (artifact timeout), 124 (command timeout) or 127 (missing binary). When every CLI at a tier exits 85 (a quota wall), it steps down one tier.

  A classified wall benches the family (`clihealth`). Five launchers called the bridge themselves and had none of this. They are the retro, the failure advisor, the phase judge, the retry adjudicator and the swarm launcher. Lanes 1676 and 1677 (2026-09-14) paid for it.

  The retro launched on codex. Codex timed out after 1800 s. The retro emitted FAIL with no disposition, and the cycle sealed abnormally.
- **A Decorator over `core.Bridge`.** `New` wraps the bridge once at the composition root (`wireOrchestratorDeps` in `cmd/evolve`), so every `Launch` gets the walk by construction.
- **No double walk.** A caller that walks its own chain (the runner, the advisor) marks each try with `BridgeRequest.ChainAttempt`. The decorator passes a marked request to the inner bridge. It does not resolve a plan. Pinned by `TestWalking_ChainAttemptPassesThrough`.
- **Shared projections.** `CLIHealthOn`, `ApplyCLIHealthBench` and `BenchOnEscalation` are the cli-health projections of the runner, moved here so both walks share one implementation.
- **The plan.** For a launch, the plan is the chain of the agent profile (`llmroute.Resolve`: primary, `cli_fallback`, tier chain, triggers). The own CLI of the caller stays the primary when it differs, so wrapping never changes which CLI runs first. It changes only what happens when that CLI fails (`TestDefaultPlanResolver_ProfileChainRunsBehindTheCallersPrimary`). A resolver with nothing to say degrades to exactly the launch that the caller asked for, once.
- **The log.** `Logf` is the diagnostic sink. The composition root points it at the stderr of the loop, so a fallback is never silent.
- **Signals pass through.** `Signals` forwards the Signal Center of the inner adapter. The verdict engine of every runner built from the wrapped handle asserts `Signals()` on its bridge. The swarm decorator and the orchestrator ask `SignalsWired()`.

  A handle that hid them will disconnect every runner from the stream, with no error. The pin in `cmd/evolve` (`TestWireOrchestratorDeps_SignalCenterReachesEveryPhaseRunner`) caught it on the first wrap. Pinned here by `TestWalking_ForwardsTheInnerSignalCenter`.

## Invariants

- **Exit 85 is the escalate exit** (`ExitUnknownPrompt`): a quota wall, a rejected model or an unanswerable prompt. `DispatchTiered` steps down a tier only when every CLI at the tier exits with it. A real bridge returns an error with a non-zero exit, so a walk advances only on an error and a trigger exit.
- **A real failure never routes elsewhere.** A legitimate FAIL (a non-trigger exit) stops the walk, because the classifier must see it (`TestWalking_RealFailureStopsTheWalk`). The caller gets the result of the walk of the runner (`WallKeeper`). That is the final try, or the quota wall of a walk that ran out after it met a wall.
- **Each wall reaches the bench.** Each exit 85 reaches the bench recorder with the CLI that hit the wall (`TestWalking_QuotaAtEveryCLIStepsDownATierAndBenchesEachWall`).
- **The bench is advice and not a veto.** `ApplyCLIHealthBench` moves candidates of an actively benched family behind the healthy ones. An entry whose bench is past due stops demoting (lazy expiry), which gives the family its canary try. Boot-timeout entries have a driver key (`ApplyDriverBench`, run first).
- **A model-group bench covers only its group.** A scoped entry demotes its family only when the first tier of the seat maps to a model of that group. The tier map is the loaded one (`bridge.LoadManifest`, live catalog included). Thus an Argon bench walks deep and top seats, and fast and balanced seats keep Gemini 3.8 Flash first. `BenchOnEscalation` writes a scoped bench only for a family in `perModelQuotaFamilies` (agy) when the `Escalation` carries a tier of its tier map. Any other wall stays family-wide.
- **Each escalation carries the tier of its try.** Every caller makes its `Escalation` with `EscalationFor(req, start)`. This function copies `BridgeRequest.Model` into `Escalation.Tier`. The walker, the runner's walk (`dispatch.go`) and the usage-evidence decorator use it.
  - **Cycle 1951 (wave 129, 2026-10-19).** The decorator and the runner's walk made an `Escalation` with no tier. Thus an exit 85 from agy at deep (Gemini 4 Argon) wrote the family bench `agy`. The next chain moved every agy seat to claude-tmux, also the balanced seats on Gemini 3.8 Flash.
  - **The fix.** Now that exit benches only `agy@Gemini 4 Argon`. A wall with no tier of the tier map still benches all of agy. A drained Gemini group on the usage screen also benches all of agy.
  - **The tests.** `TestBridge_Cycle1951AnArgonWallAtDeepBenchesOnlyTheArgonGroupAndKeepsTheBalancedAgySeatFirst`, `TestBridge_Cycle1951AnArgonWallAtDeepWalksTheDeepSeatPastArgon`, `TestBridge_Cycle1951AGeminiGroupWallStillBenchesTheWholeAgyFamily` and `TestRun_Cycle1951AnAgyExit85AtDeepBenchesOnlyTheArgonModelGroup`.

  All others have a family key (`ApplyBench`). When every candidate is benched, the order stays and a WARN says so. Each reorder is one log line.

  `cli_health.enabled: false` in `<root>/.evolve/policy.json` turns it all off (`CLIHealthOn`, X18c). No code reads `EVOLVE_CLI_HEALTH` now. Log lines are sorted so transcripts diff well.
- **Staleness guard.** `BenchOnEscalation` benches a family only when the workspace escalation report classifies a benchable wall for this dispatch. The report must name the CLI that just exited 85 and must be captured at or after the start of this dispatch. The workspace is shared across phases, so a leftover report must never bench (`TestBenchOnEscalation_BenchesTheFamilyAFreshReportNames`).

  With no report, the 85 is generic and nothing is classified. The `benched_until` value comes from the reset hint of the pane when it parses, else from the strike-scaled cooldown. A wall can need an operator fix, such as a credential wall.

  Then the log line names the fix (`credential_wall_test.go`).

- **A worktree launch never reaches a CLI without tool use.** `DefaultPlanResolver` sets `Request.NeedsTools` when `BridgeRequest.Worktree` is set, so the router drops a toolless CLI from the plan. A chain with no tool-capable CLI fails the launch and names the dropped CLIs. The inner bridge sees no call. Pinned by `TestDefaultPlanResolver_AWorktreeLaunchDropsACLIWithoutToolUseWhoeverNamedIt` and `TestWalking_AWorktreeLaunchWithNoToolCapableCLIFailsLoudlyAndLaunchesNothing`. The decision is in [internal-cliroute.md](internal-cliroute.md).

## Findings

- **Lanes 1676 and 1677**: one CLI timeout cost the work of a whole cycle. This led to the decorator.

## The model bench and the undelivered prompt (2026-10-21)

- **The model bench.** `BenchOnEscalation` benches a `model_unsupported` escalation with the scope of the model group of its tier (`seatModel`), for every family. A tier that names no model benches nothing and logs why. `scopeCoversSeat` already makes the bench stop binding once the tier resolves to another model, so a re-pin clears it.
- **The undelivered prompt.** The walk returns `llmroute.TieringExit(exit, cause)`. An `unknown_prompt` exit 85 is no quota wall: the walk still falls back to the next CLI, but it does not step down the tier.
- Pinned by `TestBenchOnEscalation_AModelUnsupportedExitBenchesOnlyThatModelOfTheFamily`, `TestBenchOnEscalation_AModelUnsupportedExitWithNoTierModelBenchesNothing`, `TestApplyCLIHealthBench_AModelBenchClearsWhenTheTierResolvesToAnotherModel` and `TestWalking_AnUnknownPromptAtEveryCLIDoesNotStepDownTheTier`. The runner walk is pinned by `TestRun_AnUndeliveredPromptIsNoWallThatTheWalkSurfaces`.
