# ADR-0045: Corrective Interaction Protocol — bounded, validated self-correction through interaction

> Status: **Proposed** (2026-06-10). Design-first; implementation in a dedicated session. Full
> evidence, threat model, component specs, and TDD plan: [interaction-protocol.md](../interaction-protocol.md).
> Builds on [ADR-0044](0044-unified-phase-recovery-protocol.md) (terminal-state recovery; this ADR
> owns the layer above death), [ADR-0037](0037-bidirectional-channel.md) (the Ask/inject plumbing),
> [ADR-0026](0026-self-healing-review-layer.md) (stop-review), PR #60 (contract corrections).

## Context

The ADR-0044 validation batch (cycles 263–269) proved the loop's interactions are point mechanisms:
contract rejection has one rung (full re-dispatch — cycle-265 burned two for a misplaced file),
corrections retry blind (no evidence of the failed attempt), unknown prompts need a human to add a
rule (cycle-267), and no injection's outcome is ever measured. Research grounding: intrinsic LLM
self-correction degrades — correction must be driven by external, unfakeable evidence (CRITIC;
TACL self-correction survey); feedback must be compact and actionable (SWE-agent ACI); pane content
is untrusted input requiring a privileged/quarantined split (CaMeL, AgentSentry, OWASP LLM Top-10).

## Decision

Adopt a **Corrective Interaction Protocol**: one owner for "repair a live or just-completed phase
through bounded, validated interaction," governed by external-evidence-only correction,
deterministic-first/LLM-last, pane-as-untrusted-input, bounded-everything, and
record-reflects-reality. Six components, each an independently shippable slice behind the existing
`EVOLVE_PHASE_RECOVERY` dial (no new flags):

| Ref | Mechanism | Pattern |
|---|---|---|
| **I1** | Interaction telemetry — every injection (nudge/auto-respond/salvage/answer/correction) records a typed Event (Rung + DecisionID + neutralized Payload) + Outcome (Result, CostUSD) via one chokepoint; records at EVERY stage incl. `off` (only ACTIONS gate) | Observer + chokepoint (C1 idiom) |
| **I2** | Graduated correction ladder — salvage (relocate-then-verify-DESTINATION, within-repo, never upgrades verdicts) → one live fix via idle-gated `KindNudge` on a NAMED session preserved through review (claude-tmux-only at v1; else skip to re-dispatch) → evidence-enriched fresh re-dispatch (aborts on exhaustion, as today). Rung re-checks are breaker-neutral (`deliverable.Verify` direct) — the GLOBAL contract-gate breaker is a separate layer touched by final outcomes only | Chain of Responsibility + Strategy |
| **I3** | AskBroker — a new pre-85 rung INSIDE the bridge's auto-respond escalate branch (the orchestrator never sees a blocked question; exit 85 already chains CLI families — that floor is untouched): closed-vocabulary KernelAnswerer answers via inbox once, quarantined advisor tail for the residue, any miss falls through to today's 85→fallback | Strategy + CoR + quarantined-LLM port |
| **I4** | Interaction-rule promotion — a thin payload specialization of I3's promote path (`recovery/promote.go` reused wholesale): RE2 + `keyspec.Classify` as a REJECTING gate + immutable healthy-corpus negative test, per-rule shadow → measured auto-enforce, boot replay re-validates against the current corpus | Reflexion promotion (ADR-0044 Slice-5 idiom) |
| **I5** | `panetrust` — the single trust boundary for pane-derived text: typed extraction for privileged decisions; capped, neutralized, untrusted-framed digests for quarantined LLM consumption AND for the persisted telemetry ledger (S10) | Facade at a trust boundary |
| **I6** | One dial — all interaction ACTIONS gate under `EVOLVE_PHASE_RECOVERY` (telemetry exempt); `EVOLVE_CHANNEL` deprecated into it | — |

Build order I1 → I2 → I5 → I3+I4 (one slice) → I6; TDD red→green per slice; shadow default is
byte-identical; I2 is RunCycle-only at v1 (resume unification is a named follow-up, the C1→C3
precedent). Echo-trap defense rides the SHIPPED mechanisms (fatal-detector preempt + once-only
injections); Progressed-span exclusion is explicitly out of v1 (new work if ever wanted).

## Consequences

- Repairable failures stop costing re-dispatches/cycles (265-class → a validated `mv`; 267-class →
  a kernel answer); the interaction registry self-expands with measured, per-rule rollout.
- Hard invariants kept: no verdict authority for any interaction component; no mid-turn interrupts
  (Busy guard); fresh-REPL isolation; trust-sensitive registries untouchable by promotion;
  salvage cannot place anything outside the deliverable contract.
- Threat model addressed explicitly (S1–S9 in the design doc): pane injection, salvage smuggling,
  rule poisoning, amplification cost, echo trap, secret exfiltration, kernel-answer abuse,
  family-wide quota livelock, double injection.

## Alternatives considered

- **In-context conversation repair** (keep the failed REPL, discuss the fix): rejected — poisoned-
  context risk; the fresh-REPL invariant plus evidence digest transfers knowledge without context.
- **Intrinsic self-correction prompts** ("re-check your work"): rejected — literature shows
  degradation without external signal.
- **Auto-respond rules straight to enforce**: rejected — a bad rule *acts*; per-rule shadow with
  measured zero false fires is the promotion gate.
- **A new env flag per capability**: rejected — no-flag-sprawl; one program dial.
