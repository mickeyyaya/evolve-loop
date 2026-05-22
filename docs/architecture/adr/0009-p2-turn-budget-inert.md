# ADR-0009: Retire P2 Turn-Budget Advisory Protocol (INERT)

**Date:** 2026-05-17
**Status:** Accepted
**Cycle:** 72

---

## Decision

Retire the `turn_budget_guidance` advisory protocol (P2) by marking it INERT. Do NOT delete the `turn_budget_guidance` field from `builder.json` — it is preserved as a source-of-truth for a future Case A watchdog.

---

## Context

P2 was shipped in cycle 70: `builder.json:turn_budget_guidance` (checkpoint at turn 15, hard-exit at turn 20) and `agents/evolve-builder.md §"Budget Checkpoint Protocol"`.

The protocol is advisory-only — the implementer (Builder) both receives the guidance and produces the telemetry that would expose a violation. Two consecutive falsifications:

- **C70:** Builder ran 64 turns vs. own 20-turn hard-exit guidance.
- **C71:** Builder ran 39 turns vs. 25-turn scout falsification ceiling (C71 artifact: `builder-usage.json num_turns=39, total_cost_usd=$0.7305`); C69 baseline was 26 turns / $0.5931 — delta +50% turns, +23% cost.

C71 retrospective §7(3) mandates: "C72 intent MUST address the P2 carryover by either (a) Case A escalation (programmatic kill in subagent-run.sh) or (b) marking P2 INERT."

`claude -p` has no `--max-turns` CLI flag (confirmed via `claude -p --help`; only `--max-budget-usd` exists). The existing adapter note at `legacy/scripts/cli_adapters/claude.sh` line 524 already acknowledges: `max-turns=$MAX_TURNS (advisory; not enforced by claude flag)`. A real-time watchdog (Case A) would require 80–120 lines of bash-3.2-compatible stdout-monitoring code + a new test harness — high regression risk to trust-kernel scripts for a single-cycle window.

---

## Rationale

Advisory-only enforcement fails when the implementer == the discloser. Two consecutive falsifications convert Case B from an acceptable enforcement surface into a deprecated one (per lesson `[[cycle-71-builder-estimate-vs-artifact]]`). INERT is the correct double-loop response: retire the failed protocol rather than ship a third attempt at the same enforcement shape.

---

## Rollback

Remove the INERT annotation from the P2 row in `docs/architecture/token-economics-2026.md`. Reinstate only if a future cycle ships a real-time watchdog that programmatically consumes `turn_budget_guidance.hard_exit_at_turn` (Case A). The `turn_budget_guidance` field in `.evolve/profiles/builder.json` is preserved as the source for that future implementation.

---

## Consequences

- `turn_budget_guidance` in `builder.json` becomes a dead field (not enforced, not deleted).
- The P2 row in `token-economics-2026.md` carries the INERT annotation with C71 telemetry citation.
- Future turn-overrun mitigation requires Case A (programmatic kill via a real-time stdout watchdog in `subagent-run.sh`), not advisory guidance.
- The `pending`+POSTHOC disclosure discipline (preventiveAction 1 from `[[cycle-71-builder-estimate-vs-artifact]]`) applies to all build-reports regardless — Builder writes `pending` for telemetry cells sourced from `*-usage.json`; Auditor reconciles post-run.
