# ADR-0104: The CLI/tier fallback chain is a property of the bridge handle, and every chain ends with every available CLI

- **Status:** Accepted (operator direction, 2026-09-14: "the fallback needs to take effect across all LLM CLIs on all phase agents"; "our policy should try all the available LLM CLIs before giving up")
- **Related:** [ADR-0097](0097-worktree-fence.md) (the read-only fence the retro holds around its own launch), [ADR-0103](0103-component-breakdown-program.md) (unit 11, the phase runner whose walk this generalises), [ADR-0101](0101-signal-center.md) (the Center the wrapped handle must keep forwarding)

## Context

The shared phase runner has walked a CLI chain since WS-G1 (`llmroute.Dispatch`) and a tier chain since WS-876 (`DispatchTiered`): on a trigger exit — 80 REPL boot timeout, 81 artifact timeout, 124 command timeout, 127 missing binary — it advances to the next CLI; when every CLI at a tier exits 85 (a quota wall, a rejected model, an unanswerable prompt) it steps down a tier; a classified wall benches the family (`clihealth`). Five launchers never entered that walk because they call the bridge themselves: the retro, the failure advisor, the phase judge, the retry adjudicator and the swarm launcher.

Wave 2 of the pipeline-health verification (2026-09-14) showed the cost. Lanes 1676 and 1677 reached their retrospectives on `codex-tmux@deep`; the account rejected the model, the pane parked, and after the 1800 s artifact window the retro emitted FAIL with no disposition — the cycle sealed abnormally (`ORCHESTRATOR_CYCLE_FAILED … disposition.json is a required retro deliverable but is absent`). One CLI's timeout at the last phase threw away two hours of lane work, while the build phase of the same lanes had fallen back to Claude on the identical exit code because the runner walks and the retro does not.

Two further gaps sat behind it. The universal fallback appended the host's other CLIs only when the whole configured chain was *absent* — a chain that was present but walled ended the walk. And eleven claude-primary agent profiles carried no `cli_fallback` at all, two of them (`auditor`, `tdd-engineer`) on the mandatory spine.

## Decision

1. **The walk belongs to the bridge handle.** `internal/bridgechain.Walking` is a Decorator (GoF) over `core.Bridge`. Wrapped ONCE at the composition root (`cmd/evolve wireOrchestratorDeps`), it resolves a plan for every `Launch` — the agent profile's primary, `cli_fallback`, tier chain and triggers (`llmroute.Resolve`), the caller's own CLI kept as the primary, a capability probe, the cli-health bench, the universal tail — and walks it with `DispatchTiered`. Every consumer receives the wrapped handle. A caller that walks its own chain (the runner, the advisor) marks each attempt with `BridgeRequest.ChainAttempt` and is passed straight through, so no launch is walked twice; a launch without the mark is a caller that never heard of the chain and gets the walk by construction. The handle forwards the adapter's `Signals()` / `SignalsWired()` so the Center still reaches every runner. A source-scan test pins that the raw adapter is only constructed, configured through its `Set*` methods, wrapped, handed to the catalog publisher as its resolver sink and exposed on `orchDeps` — never launched.
2. **Every chain ends with every available CLI.** `llmroute.ApplyUniversalFallback` now appends the discovered, profile-allowed CLIs after the configured chain unconditionally (the configured chain keeps precedence: it runs first, in order). Both walks apply it through the same discovery closure.
3. **The agy ban is config.** `policy.json` `workflow.universal_fallback_exclude` (default `["agy"]`, the 2026-06-07 operator judgment) filters the last-resort tail; `[]` lifts it. A banned family may still be a profile's configured primary.
4. **Every agent profile names a chain.** The eleven claude-primary agents get one. The five Claude-family-floor agents (`auditor`, `adversarial-review`, `tdd-engineer`, `spec-verifier`, `spec-verify`) fall back IN-FAMILY only (`claude-p`) and keep `allowed_clis: ["claude"]`: the anti-gaming floor (`family_floor_test.go`) outranks throughput — codex must never grade or test codex — so a fully walled Claude family still fails those phases loudly. The six runtime-minted audit-side stubs get `["claude-p", "codex-tmux"]` on the plane; `TestEveryAgentProfileHasAFallbackChain` guards the tracked tree.

## Consequences

- A phase gives up only after every CLI in its chain, at every tier of its tier chain, has failed — the cost of a walled CLI is one fast attempt, not a phase.
- The retro's failure-learning disposition survives a walled primary; the abnormal seal is reachable only when every CLI is exhausted.
- The cli-health projections (`CLIHealthEnabled`, `ApplyCLIHealthBench`, `BenchOnEscalation`) have ONE home, `bridgechain`; the runner calls them under its own log prefix.
- Skill overlays are resolved once per launch by the direct launchers (the runner still resolves them per attempt); a tier step-down on a direct launch keeps the primary attempt's overlays — accepted for this slice.
- Operators who want cross-family rescue on the floored phases must change the floor deliberately (`family_floor_test.go` and the profiles together); this ADR does not.
