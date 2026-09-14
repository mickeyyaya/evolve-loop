# 2026-09-14 — a retro's CLI timeout sealed the cycle: the retro never entered the fallback walk

**Class:** pipeline (one launcher's missing fallback turned a CLI wall into an abnormal seal).
**Surface:** `internal/phases/retro/retro.go` (direct `bridge.Launch`), and four sibling launchers: `core/failure_advisor.go`, `core/phase_judge.go`, `core/retry_adjudicator_agent.go`, `phases/swarmrunner`.
**Found by:** the operator, wave 2 — "if retro codex timeout, it should fallback to claude, not seal directly!!"

## What happened

Both wave-2 lanes reached their retrospectives after their ships failed (the gate-environment leak, #615). The retro profile pins `codex-tmux` at `deep`; the account rejects that model (F7). The pane parked, the artifact window ran its 1800 s, and the bridge exited 81. The retro handled it as its GAP-9 self-healing path prescribes — "emitting FAIL verdict; orchestrator routes via failure-adapter (non-fatal)" — but the failure-learning disposition the retro owes was never written, and the cycle sealed: `ORCHESTRATOR_CYCLE_FAILED … retro_decision=failure-learning: disposition-gate: disposition.json is a required retro deliverable but is absent … termination_reason=cycle aborted in phase retro (abnormal-exit epilogue)`. Two lanes, two hours of build work each, no disposition, no requeue decision.

The build phase of the same lanes had met the same exit code on the same CLI and simply continued: `fallback 2: trying cli=claude-tmux tier=deep (previous=codex-tmux@deep=81 exit=81)`.

## Root cause

The shared phase runner dispatches through `llmroute.DispatchTiered` — the chain walk that advances on 80/81/124/127, steps down a tier on 85 and honours the cli-health bench. The retro resolves its CLI by hand (`EVOLVE_CLI` → profile → `claude-tmux`) and calls `p.bridge.Launch` once, with one retry for delivery failures only. No chain, no tiers, no bench — it dispatched on codex while codex was already benched. Four other launchers do the same.

Two gaps compounded it: the universal fallback appended the host's other CLIs only when the configured chain was absent, never when it was present but walled; and eleven claude-primary profiles had no `cli_fallback` at all (auditor and tdd-engineer among them), so a Claude wall on those phases ended the phase the same way.

## Fix (this change) — ADR-0104

`bridgechain.Walking`, a Decorator over `core.Bridge` wrapped once at the composition root: every `Launch` resolves the agent's plan and walks it; the runner and the advisor mark their own attempts (`BridgeRequest.ChainAttempt`) and pass through. The universal tail is appended unconditionally; the agy ban becomes `workflow.universal_fallback_exclude` (default `["agy"]`); the eleven profiles get chains — in-family only for the five Claude-family-floor agents, whose anti-gaming floor outranks throughput.

Red first: `TestWalking_ArtifactTimeoutAdvancesToTheNextCLI` and `TestRun_TimeoutOnThePrimaryCLIFallsBackThroughTheChain` (the retro, through the handle, on the lane-1676 shape: codex 81 → claude PASS); the composition scan failed with "wrapped exactly once (got 0)"; the universal-fallback no-op test was flipped before the semantics changed; `TestEveryAgentProfileHasAFallbackChain` failed on eleven profiles. The first wrap hid the adapter's `Signals()` accessor and `TestWireOrchestratorDeps_SignalCenterReachesEveryPhaseRunner` caught it — the handle now forwards it.

## Cost on this wave

Two retros × 30 min on a dead model, and two cycles sealed without the failure-learning disposition that decides requeue vs. poison — the items went back to the inbox root by the epilogue's default, not by judgment.

## Operator notes

- `[bridge-chain] agent=<x> fallback N: trying cli=… (previous=…@…=<exit> exit=<exit>)` on stderr is a direct launcher walking its chain; `[runner] phase=<x> fallback N: …` is the runner's own walk. Both end in `tier step-down` lines when every CLI at a tier walled.
- A cycle that seals with `disposition.json … absent` after this change means EVERY CLI in the retro's chain failed — read the chain lines above it.
- The five floored agents keep their chains inside the Claude family on purpose. If Claude's whole family is walled at every tier, those phases fail loudly rather than let the builder's family judge its own work; lifting that is a deliberate change to `family_floor_test.go` and the profiles together.
