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
- **Shared projections.** `CLIHealthEnabled`, `ApplyCLIHealthBench` and `BenchOnEscalation` are the cli-health projections of the runner, moved here so both walks share one implementation.
- **The plan.** For a launch, the plan is the chain of the agent profile (`llmroute.Resolve`: primary, `cli_fallback`, tier chain, triggers). The own CLI of the caller stays the primary when it differs, so wrapping never changes which CLI runs first. It changes only what happens when that CLI fails (`TestDefaultPlanResolver_ProfileChainRunsBehindTheCallersPrimary`). A resolver with nothing to say degrades to exactly the launch that the caller asked for, once.
- **The log.** `Logf` is the diagnostic sink. The composition root points it at the stderr of the loop, so a fallback is never silent.
- **Signals pass through.** `Signals` forwards the Signal Center of the inner adapter. The verdict engine of every runner built from the wrapped handle asserts `Signals()` on its bridge. The swarm decorator and the orchestrator ask `SignalsWired()`.

  A handle that hid them will disconnect every runner from the stream, with no error. The pin in `cmd/evolve` (`TestWireOrchestratorDeps_SignalCenterReachesEveryPhaseRunner`) caught it on the first wrap. Pinned here by `TestWalking_ForwardsTheInnerSignalCenter`.

## Invariants

- **Exit 85 is the escalate exit** (`ExitUnknownPrompt`): a quota wall, a rejected model or an unanswerable prompt. `DispatchTiered` steps down a tier only when every CLI at the tier exits with it. A real bridge returns an error with a non-zero exit, so a walk advances only on an error and a trigger exit.
- **A real failure never routes elsewhere.** A legitimate FAIL (a non-trigger exit) stops the walk, because the classifier must see it (`TestWalking_RealFailureStopsTheWalk`). The caller gets the result of the walk of the runner (`WallKeeper`). That is the final try, or the quota wall of a walk that ran out after it met a wall.
- **Each wall reaches the bench.** Each exit 85 reaches the bench recorder with the CLI that hit the wall (`TestWalking_QuotaAtEveryCLIStepsDownATierAndBenchesEachWall`).
- **The bench is advice and not a veto.** `ApplyCLIHealthBench` moves candidates of an actively benched family behind the healthy ones. An entry whose bench is past due stops demoting (lazy expiry), which gives the family its canary try. Boot-timeout entries have a driver key (`ApplyDriverBench`, run first).

  All others have a family key (`ApplyBench`). When every candidate is benched, the order stays and a WARN says so. Each reorder is one log line.

  `EVOLVE_CLI_HEALTH=0` turns it all off. Log lines are sorted so transcripts diff well.
- **Staleness guard.** `BenchOnEscalation` benches a family only when the workspace escalation report classifies a benchable wall for this dispatch. The report must name the CLI that just exited 85 and must be captured at or after the start of this dispatch. The workspace is shared across phases, so a leftover report must never bench (`TestBenchOnEscalation_BenchesTheFamilyAFreshReportNames`).

  With no report, the 85 is generic and nothing is classified. The `benched_until` value comes from the reset hint of the pane when it parses, else from the strike-scaled cooldown. A wall can need an operator fix, such as a credential wall.

  Then the log line names the fix (`credential_wall_test.go`).

## Findings

- **Lanes 1676 and 1677**: one CLI timeout cost the work of a whole cycle. This led to the decorator.
