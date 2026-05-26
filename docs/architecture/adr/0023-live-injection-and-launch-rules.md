# ADR-0023: Live command injection + launch-time system-prompt for agent-bridge

> Status: **Proposed** (2026-05-26). Adds a runtime control channel into already-running tmux-REPL
> agents (facet A) and a CLI-agnostic launch-time rules prepend (facet B). Builds on ADR-0022
> (LaunchIntent → Realizer) and reuses the existing artifact-wait poll loop + policy-injection seam.

## Context

After `bridge.Launch()` hands a prompt to a CLI, the launch is fire-and-forget: the orchestrator
blocks until the agent produces its artifact, with no way to influence the agent mid-run. Two needs:

1. **Live command injection (facet A):** send an urgent update — a correction, a new instruction,
   or an interrupt — into an *already-running* agent without killing and relaunching it.
2. **Launch-time system prompt / rules (facet B):** set per-agent system-level rules at launch
   (e.g. "adversarial-audit mode", project guardrails).

The substrate for facet A already exists. The `*-tmux` REPL drivers keep a persistent tmux session
alive and drive it via `tmux send-keys` / `load-buffer` + `paste-buffer`. The driver **already runs
a 2-second poll loop while the agent works** (`driver_tmux_repl.go`, artifact-wait + auto-respond).
The dead `Realization.REPLInput []string` field (ADR-0022) was declared for post-boot injection but
never wired.

## Decision

### Facet A — file-based inbox drained in the existing poll loop

| Axis | Decision |
|---|---|
| Transport | Append-only NDJSON inbox at `<workspace>/.bridge-inbox/<agent>.ndjson`. No new ports/sockets; durable + auditable; fits the unified-envelope model. |
| Atomicity | Pure-Go `O_APPEND` + single sub-4096-byte `Write` (POSIX-atomic for N concurrent writers). Same pattern `phaseobserver.emit` uses. |
| Cursor | Byte-offset `Cursor.Drain` delivers only complete lines; **seeks to EOF on driver entry** so a resumed named session (or stale ephemeral file) never replays a pre-launch backlog — only post-launch envelopes are injected. |
| Drain site | Inside the existing artifact-wait poll loop, **before** the auto-respond tick (so an interrupt pre-empts a pending auto-reply). **No async refactor, no orchestrator change, no process handle in `BridgeResponse`** — the blocking Launch model is untouched. |
| Inject timing | `command`/`nudge`/`system_rule` are **idle-gated** (injected only when the prompt marker is visible); a mid-turn arrival is re-queued, bounded by `maxInjectDefer` (10) then dropped with a WARN. `interrupt` sends ESC + settle, then injects regardless of state. |
| Delivery | Via the paste buffer (preserves multi-line/special chars — `SendKeys` would mangle them) through a dedicated `<agent>-inject.txt` scratch file (no collision with `resolved-prompt.txt`). |
| Senders | `evolve bridge send --workspace --agent [--kind] [--source] <body>` (operator/scriptable) **and** the phase-observer soft-stall nudge (opt-in). Both use the same `inbox.Append`. |
| Scope | **Go-tmux-only by physics.** Headless drivers (`claude -p`, `codex exec`, `agy -p`) exit after one prompt — nothing to inject into. `bridge send` to a headless agent accumulates an undrained file (harmless). The production default routes phases to tmux-REPL drivers, so the live workload is covered. |

`Kind` vocabulary: `command` | `interrupt` | `nudge` | `system_rule`.

### Facet B — launch-time rules prepended at the policy seam

| Axis | Decision |
|---|---|
| Field flow | `core.BridgeRequest.SystemPrompt` ← runner-resolved from `profiles.Profile.SystemPrompt` / `SystemPromptFile`. |
| Resolution | `systemprompt.Resolve` mirrors `resolvePolicy` precedence: `EVOLVE_<AGENT>_SYSTEM_PROMPT > EVOLVE_SYSTEM_PROMPT > profile.system_prompt > read(system_prompt_file) > ""`. |
| Application | `injectRulesPrefix` prepends a `## Rules` block at the **same adapter seam** as `injectPolicyPrefix` (both Go and bash paths). Order: rules < policy < body. |
| Why the seam, not a flag | `launchCmdLine` has no shell-quoting; a multi-line system prompt routed through a `--append-system-prompt` *launch flag* would corrupt the tmux launch command. The prompt-prepend is CLI-agnostic (headless + tmux) and sidesteps quoting entirely. |

## Consequences

- **Recovery before kill:** the phase-observer can nudge a soft-stalled agent (opt-in via
  `EVOLVE_OBSERVER_NUDGE_S`) before the hard SIGTERM.
- **Operator control mid-run:** any process that can write the workspace can steer a live agent.
- **No new transport surface:** files only; no sockets, no orchestrator restructuring.
- **Bounded mischief:** mid-turn commands are idle-gated and defer-bounded; `interrupt` is explicit.

### Deviation logged

The implementation made the observer nudge **opt-in (`EVOLVE_OBSERVER_NUDGE_S`, default 0 = off)**
rather than the originally-sketched `StallS/2` default — it changes runtime behavior (writes to the
drained inbox), so it only fires when an operator enables it (no-feature-flag-sprawl posture).

## Files

- `go/internal/bridge/inbox/{inbox,writer,reader}.go` — envelope + atomic append + cursor
- `go/internal/bridge/driver_tmux_repl.go` — drain + `injectEnvelope`/`injectText`, REPLInput seed
- `go/cmd/evolve/cmd_bridge.go` — `bridge send` subcommand
- `go/internal/phaseobserver/phaseobserver.go` + `go/cmd/evolve/cmd_phase_observer.go` — nudge hook
- `go/internal/systemprompt/`, `core/ports.go`, `profiles/profiles.go`, `phases/runner/runner.go`,
  `adapters/bridge/bridge.go` — facet B
