# Agent-Bridge Dispatch Contract

> Status: living doc for the agent-bridge hardening program (plan: `sleepy-wandering-salamander`).
> Covers the invariants that make every subagent/phase-agent dispatch go through the one bridge
> process, safely and at scale. B1 + B2 landed; B3–B6 will extend this doc.

## Request

The agent-bridge (`evolve subagent run` / `evolve subagent dispatch-parallel`) must be the **only**
path that dispatches a phase agent or subagent — the in-process Agent/Task tool is banned. Three
structural weaknesses surfaced while fixing the sandbox-confinement bug:

1. The bridge-only invariant was not enforced — a `LEGACY_AGENT_DISPATCH` escape hatch fell back to
   the in-process tool.
2. Recursion (fan-out workers re-invoking the binary) was implicit and unbounded, and its
   sandbox-coherence under nesting was unproven.
3. The output contract / verification / concurrency guarantees were under-tested.

## B1 — Bridge-only dispatch invariant

**Problem.** `LEGACY_AGENT_DISPATCH=1` made `evolve subagent run` print a *"fall back to in-process
Agent tool"* instruction plus a `LEGACY_DISPATCH` stdout token. That token was a **contract signal**
(not a Go branch) telling the LLM orchestrator to dispatch in-process — the exact path that must be
banned.

**Alternatives considered.**
- *Silently ignore the flag (always bridge).* Rejected — a silent change (Rule 3); a caller setting
  the flag would get surprising behavior with no signal.
- *Loud, default-off deprecation with a removal date.* Reasonable, but leaves the banned path
  reachable during the deprecation window.
- *Hard error (chosen).* Setting the flag fails loudly at the single `Run()` chokepoint. No dispatch
  can reach the in-process tool, and the failure is self-describing.

**Design.** `ErrInProcessDispatchBanned` sentinel + one `enforceBridgeOnly(legacyRequested)` predicate,
called early in `Run()` (before profile/CLI resolution) — the one chokepoint all dispatch (single,
fan-out, recursive) funnels through. `RunResult.LegacyDispatch` and the `cmd_subagent` fallback
block/token were deleted. A source-guard test pins that no in-process fallback signal (or
"honored-env" advertisement of the retired flag) can regress.

The agent allow-list was collapsed to an `agentRoles[]` SSOT; `agentRolePattern` derives from it and
the all-roles invariant test iterates it, so a new role is added in exactly one place.

## B2 — Recursion through the same bridge

**Problem.** A fan-out worker recurses by running `evolve subagent run <agent>-worker-<subtask>` —
re-entering the same binary's `Run()` (good: never the in-process tool). But (a) the command was
built inline and untested, (b) the recursion was unbounded, and (c) a recursive agent is
*nested-Claude inside nested-Claude*, so the Part A sandbox-confinement SSOT must stay coherent at
every depth.

**Design.**
- **Tested recursion command.** Extracted pure `buildWorkerRecursionCommand()` — composes
  `PROMPT_FILE_OVERRIDE=… CLAUDECODE_TYPE= EVOLVE_DISPATCH_DEPTH=<n+1> <bin> subagent run
  <role>-worker-<subtask> <cycle> <ws>`. A conformance test proves it re-enters the bridge dispatch
  path for every worker.
- **Depth cap.** `EVOLVE_DISPATCH_DEPTH` threads recursion depth across each bridge re-entry
  (`ReadDispatchDepth` parses it; absent ⇒ 0). `enforceDispatchDepth` rejects past `maxDispatchDepth`
  (3) at both the `Run()` and `DispatchParallel()` entry points (one predicate). A fan-out loop can't
  recurse unboundedly.
- **Sandbox coherence by construction.** The worker command clears `CLAUDECODE_TYPE` — a recursive
  child is, by definition, **not** the top-level host. If a stale `CLAUDECODE_TYPE=host` leaked into a
  child, `sandbox.DetectNested` would return false → `ShouldWrap` would attempt the inner OS sandbox →
  on macOS `sandbox_apply()` EPERMs and hangs the REPL (the Part A failure mode). Since `claude`
  self-sets `CLAUDECODE` in every process, clearing the host marker keeps `DetectNested=true` at any
  depth → no inner wrap.

**Why not also strip/inject `CLAUDECODE` itself?** Unnecessary — `claude` sets it in every claude
process, so the nested signal is already present at every depth. Only the *host* marker can wrongly
suppress nesting, so only it is cleared.

## Invariants (enforced by tests)

| # | Invariant | Enforced by |
|---|-----------|-------------|
| I1 | No dispatch reaches the in-process Agent tool | `enforceBridgeOnly` @ `Run()`; source-guard test |
| I2 | Agent allow-list is single-sourced | `agentRoles[]` → `agentRolePattern`; all-roles test |
| I3 | Fan-out workers recurse via `subagent run` | `buildWorkerRecursionCommand` + conformance test |
| I4 | Recursion depth is bounded | `EVOLVE_DISPATCH_DEPTH` + `enforceDispatchDepth` (cap 3) |
| I5 | Recursive children stay nested (no inner wrap) | `CLAUDECODE_TYPE` cleared per worker; sandbox test |

## Out of scope (later phases)

B3 (output-contract verification program + progress-aware artifact deadline), B4 (conformance suite),
B5 (concurrency `-race` stability), B6 (registry-driven allow-list + session reaping).
