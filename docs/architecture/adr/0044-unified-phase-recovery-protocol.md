# ADR-0044: Unified Phase Recovery Protocol — deterministic-first, single-owner recovery

> Status: **Proposed** (2026-06-09). Design-first: records the analysis and chosen direction; implementation is
> risk-ranked and gate-tested (no blind change to the hot dispatch loop). Full evidence + design:
> [phase-recovery.md](../phase-recovery.md). Builds on [ADR-0026](0026-self-healing-review-layer.md) (stop-reviewer),
> [ADR-0029](0029-cli-fallback-chain-and-per-agent-overrides.md) (CLI fallback), [ADR-0039](0039-failure-floor-and-failure-signal-contract.md) (ship repair ladder + failure floor).

## Context

Post-mortem of cycle-262 (2026-06-09): a cycle whose work was **done correctly** recorded itself as a
**failure**. Three failures stacked — codex self-upgraded its binary mid-phase (D6), retro was dispatched with
an invalid `claude --model auto` (D3), and the build's **successful** CLI fallback (`codex exit 81 → claude
exit 0`, valid `build-report.md` PASS, goal achieved) was **never reconciled into orchestration** (no
`build-usage.json`, no `phase-timing` entry, no verdict) → audit skipped → cycle FAIL (D1). ~40 min of a ~52 min
cycle was burned waiting out the `maxExtends` backstop on two **self-describing fatal pane states** that nothing
detects (D2). Full taxonomy (D1–D7) in the design doc.

**Root cause:** recovery is an **un-owned cross-cutting concern**, smeared across `bridge/driver_tmux_repl.go`,
`phases/runner`, `core/orchestrator.go`, `core/observer.go`, and `phases/ship` (repair ladder). Each layer
assumes another catches the failure; none owns *"did this phase succeed, and is that success recorded?"* — so
the record can diverge from reality.

## Decision

Adopt a **Unified Phase Recovery Protocol** with one owner (a *Phase Recovery Pipeline*), governed by
**deterministic-first, LLM-last** (Core Agent Rule 5) and **outcome-orientation** (reconcile to a recorded
verdict, never "process exited 0"). Built from standard patterns:

| Ref | Mechanism | Pattern | Fixes |
|---|---|---|---|
| **C1** | Single-source verdict reconciliation — every terminal path (primary, fallback, timeout) funnels through one `reconcile(deliverable, dispatch)` chokepoint that records verdict/usage/timing | DRY chokepoint (extends `deliverable.Verify`) | **D1** |
| **C2** | `ClassifyTerminal(pane, exit, artifact) → TerminalCause`; known-fatal causes fast-fail; `ModelFlagPolicy` normalizes `auto` per driver | Template Method hook + Strategy | **D2, D3** |
| **C3** | Recovery handlers in an ordered chain: `ModelFlagPolicy → FatalPaneDetector → Fallback → VerdictReconciler → LLMFailureAdvisor` | Chain of Responsibility | composes **D1–D4** |
| **C4** | Observer stays detection-only; a `StallPolicy` maps events → `extend\|kill+retry\|escalate` | Observer + Strategy/Policy (SRP) | **D5** |
| **C5** | Preflight predicate: any `-tmux` CLI with `startup update check = true` must be pinned (converge) or Halt | Specification | **D6** |

The **LLM failure-advisor is the escalation tail only** (unclassified states); its classification of a novel
pane signature is **promoted into the deterministic registry** `FatalPaneDetector` reads — the known-failure
set grows, the LLM never sits on the hot path for a known failure.

Implementation is risk-ranked (design doc §7): **C1 first** (alone it makes 262-class cycles PASS), then C2
(model-flag + fatal-detector), then C5+D4 (config), C4, and finally the C3 chain refactor that composes them.

## Consequences

- **Positive:** a successful recovery is *structurally* recorded (D1 can't recur); self-describing fatal states
  fast-fail instead of burning the `maxExtends` backstop (D2); recovery gains a single testable owner; the
  deterministic catch-set self-expands via the LLM tail.
- **Constraint (non-negotiable):** phase isolation — no cross-phase REPL/context reuse; builder≠auditor
  cross-family floor unchanged.
- **Constraint:** reconciliation only ever upgrades a synthesized FAIL toward the agent's real verdict; it never
  invents a PASS. A valid deliverable with red EGPS predicates still fails the gate.
- **Discipline:** no blind change to the hot dispatch loop — each step is TDD'd, adversarially audited, and
  behavior-neutral where possible.

## Alternatives considered

- **Per-cycle `brew pin`/`unpin`-on-exit.** Rejected as primary: unpin depends on a clean exit (a kill/OOM/reboot
  never runs it → host CLI silently frozen); `brew pin` is global + persistent, so concurrent cycles race.
  Superseded by C5 (pin as convergent steady state; updates are an explicit operator action).
- **Raise `maxExtends` / lower the artifact timeout.** Treats the symptom, not the untyped-terminal-state cause.
- **Route all failures through the LLM failure-adapter.** Violates deterministic-first; the LLM retro itself
  failed in cycle-262.
- **Add CLI fallbacks everywhere and stop.** Insufficient without C1 — an unrecorded *successful* fallback still
  fails the cycle.
