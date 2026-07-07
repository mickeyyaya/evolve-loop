# Plan: Token-Usage Telemetry (per-phase + advisor + subagent) → Concurrency Token-Optimization Campaign

> Supersedes the completed workspace-hygiene plan (preserved at docs/plans/workspace-hygiene-2026-07.md).

## Context

Token/quota burn blocks work — worst under concurrency (2 lanes) and provider rate-benching (codex benched 83% of last batch → all burn on claude). Operator directive: **measurement first** — "build the feature that monitors the token usage of each phase including orchestrator and advisor to better locate the max token usage"; research (2026 papers + OSS) stored as documented reports and turned into actionable tasks; **plan mode for design, evo loop for implementation**; strict TDD/clean-code/design-patterns; never stop the loop.

**Why measurement first**: every token field in the system is zero today under the default tmux drivers (`<phase>-usage.json` cost_usd=0; `BridgeResponse.Tokens` never populated). Local proxies show ~164 CLI session boots per 8-cycle batch, ~12 phase + advisor LLM calls/cycle, scout 5× any other phase, 20 fallback double-dispatches and 40 quota-exhaustion events per batch — but nothing attributes real tokens to phases. Optimization without a baseline would be blind.

## Design facts (verified by 3 Explore passes + 1 adversarial Plan review)

- **True single chokepoint = `bridge.Engine.Launch` (`go/internal/bridge/engine.go:323`)**. Every LLM call funnels through it: phase runners (runner.go:529), retro, swarm workers (swarmrunner.go:257), advisor Plan/RePlan/Propose (`advisorLaunch`, phase_advisor.go:250→277-293), failure-advisor (failure_advisor.go:115), plan-judge (phase_judge.go:67, unwired), campaign subagents, AND `evolve subagent run` (validateprofile.go:327-360 constructs the engine directly — bypasses the adapter, so the ADAPTER is NOT sufficient).
- **Canonical counts type exists and is wire-pinned**: `cyclestate.TokenUsage{Input, Output, CacheRead, CacheWrite}` (cyclestate/result.go:18-24; pinned by `TestTokenUsage_Wire`). `BridgeResponse.Tokens`/`CostUSD` (ports.go:293-294) and `PhaseResponse.Tokens` (phase.go:167) exist, zero-initialized. Runner already copies bres.Tokens→PhaseResponse on all return paths (runner.go:611-683). Do NOT mutate the counts type — provenance (source/cli/model/session) lives in the collector's own record.
- **Gaps that drop tokens today**: `recovery.PhaseOutcome` has CostUSD but no Tokens (outcome.go:44-47 → `phaseOutcomeFrom` drops them); swarm `LaunchResult` sums only CostUSD (swarmrunner.go:142,266); subagent `Result` drops them (subagent.go:70,231).
- **Per-attempt attribution is only possible at the Launch seam**: the CLI-fallback chain overwrites `bres` per candidate (runner.go:523-570; same in advisorLaunch) and the orchestrator retry loop overwrites per attempt (cyclerun_dispatch.go:164-171); `recordPhaseOutcome` (failure_learning.go:92-151) sees only the terminal response + attempt_count. 16+ terminal record sites — do NOT thread attempt arrays through them.
- **Ledger is the wrong sink** for telemetry: custom UnmarshalJSON wire struct silently drops unmapped fields (ports.go:129-223) + hash-chain bloat. Workspace ndjson sidecar is the right family.
- **Transcript reality (claude-tmux)**: pane cwd = worktree (driver_tmux_repl.go:110-121); fleet lanes have distinct worktrees → distinct `~/.claude/projects/<slug>/` dirs; **swarm readers share one worktree** → same slug dir, overlapping windows (the collision case); phase retries share the worktree but per-Launch windows are disjoint. Transcript lines carry their own `cwd`/`sessionId`, and every launch prompt embeds a unique artifact path (bridge.go:284-300) usable for content verification. Honor `CLAUDE_CONFIG_DIR`; treat slug format as an untrusted external contract.
- Aggregation rails exist: `phasetiming.Entry` (has CostUSD, no tokens) → `dossier.Build` projection; `cyclecost.PhaseCost` already has the 4 token fields (events-parse source, starved under tmux); `budgethistory.Collect` walks last-N cycles → `fleetbudget.Plan`.
- **No policy toggle**: always-on, fail-open (a read-only telemetry toggle would be feature-flag smell; add an ObserverPolicy-style block only when a real knob emerges).

## Slices (each cycle-sized, RED-first, apicover-named; loop-implemented)

| # | Content | Deps |
|---|---|---|
| **S1** | `internal/tokenusage` transcript scanner: config-root resolution (HOME + CLAUDE_CONFIG_DIR), slug-dir glob + **per-line cwd verification**, per-Launch window, usage summing with **message-id dedup**, content-verification tie-breaker (artifact path in first user message), source-confidence tags (`transcript` / `transcript-ambiguous` / `none`) | — |
| **S2** | Collector chain (fidelity order: transcript > eventsResult > scrollbackPeak) + refactor `cyclecost.parseEventsLog` per-phase extraction into a shared reusable func (no duplication) | S1 |
| **S3** | **`Engine.Launch` instrumentation**: populate `BridgeResponse.Tokens` + append one record per Launch to workspace `llm-calls.ndjson` `{ts, agent, phase, cli, model, attempt, tokens{in,out,cache_r,cache_w}, source, duration_ms, exit_code}` — per-attempt waste attribution falls out free; fail-open (collector error never fails a Launch) | S2 |
| **S4** | Terminal projection: add `Tokens` to `recovery.PhaseOutcome` + `phaseOutcomeFrom` + `phasetiming.Entry` + `<phase>-usage.json` sidecar; legacy read-compat for old artifacts | S3 |
| **S5** | Non-phase attribution: `AdvisorSpan` gains tokens (phase_advisor.go:330-338); swarm `LaunchResult` + subagent `Result` stop dropping tokens | S3 |
| **S6** | `phasetiming.Rollup` token rollups (totals, ByArchetype, cache-hit ratio = cache_read/(input+cache_read), wasted-on-failed-attempts from llm-calls.ndjson) + dossier `PhaseRecord` tokens | S4 |
| **S7** | `evolve tokens report [--last N]` (reuse budgethistory cycle walk): top consumers by phase/site/cycle/lane, cache-hit trend, waste class; `cyclecost` switches to sidecar-preferred source (events-parse fallback) | S6 |
| **S8** | Fleet join (shadow): `budgethistory.Throughput` gains median tokens/cycle (native units, never $); shadow-logged join with `quotastate.TightestRemaining` via existing `fleet.budget` spine — groundwork for admission control, zero behavior change | S6 |

### RED tests (verbatim)
- S1: `TestTranscriptScan_SumsUsageWithinWindow` · `TestTranscriptScan_DeduplicatesStreamedUsageByMessageID` · `TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted` · `TestTranscriptScan_MissingDirYieldsSourceNone`
- S2: `TestCollectorChain_FidelityOrderFirstNonEmptyWins`
- S3: `TestEngineLaunch_PopulatesBridgeResponseTokens` · `TestEngineLaunch_AppendsLLMCallRecordPerFallbackAttempt` · `TestEngineLaunch_CollectorErrorNeverFailsLaunch`
- S4: `TestRecordPhaseOutcome_ProjectsTokensToSidecarAndTiming` · `TestSidecar_LegacyWithoutTokensParses` · `TestTimingEntry_LegacyLogParses`
- S5: `TestAdvisorSpan_CarriesTokenUsage` · `TestSwarmMerge_SumsWorkerTokens` · `TestSubagentRun_RecordsUsage` (covers the adapter-bypass path)
- S6: `TestSummary_TokenRollupAndCacheHitRatio` · `TestBuild_ProjectsPhaseTokens`
- S7: `TestTokensReport_RanksPhasesByInputTokens` · `TestCycleCost_PrefersSidecarOverEvents`
- S8: `TestCollect_MedianTokensPerCycle` + fleetbudget shadow-log assertion

## Research base → stored deliverables (operator lands these after approval)

1. `knowledge-base/research/token-concurrency-2026/` — `README.md` (local burn evidence + priority translation), `papers.md` (18 sources: HiveMind admission −48-100% dead-agent waste/−96% eval cost; DeLM shared context −50%; "Don't Break the Cache" −41-80%; cache reads ITPM-exempt ≈5× effective quota; TokenDance 11-17× duplicated context; scoped contexts −82%; AGENTS.md files +>20% cost), `oss.md` (15 projects: SWE-agent masking −52.7%; aider repo-map/keepalive; OpenHands condenser; RTK −89%; Claude Code --resume/context-editing −84%; opencode uncached-tokens gotcha).
2. `docs/plans/token-telemetry-2026-07.md` — this design (loop-consumable).
3. Inbox items: S1 0.95 → S8 0.88 (deps noted) + phase-2 optimization items at 0.72-0.80 (gated on S7 baseline): cache-discipline audit, session-resume-on-retry, RTK-style tool-output compression hook, Go repo-map artifact, handoff digests, role-scoped instruction digests, tool-call memoization.

## Also at next batch boundary (operator, pre-approved chores)
Reconcile diverged local main (quiet tree: clear stray `.tmp.37299` from index — tracked-junk class, staging-guard item exists — then `git merge origin/main --no-edit`, no push; next lane ship pushes FF) → verify next main go CI run green (closes the CI investigation).

## Verification
- Per slice: `go test -race` on touched pkgs + repo-wide vet/gofmt + apicover -enforce (CI-parity); loop's own gates apply.
- End-to-end acceptance (after S7 soaks one batch): `evolve tokens report --last 8` shows non-zero per-phase tokens with source=transcript for claude-tmux phases, advisor calls attributed, per-attempt waste visible for any fallback chain, cache-hit ratio computed; dossiers carry token blocks; then phase-2 optimization items get their baseline weights re-ranked by MEASURED top consumers.
