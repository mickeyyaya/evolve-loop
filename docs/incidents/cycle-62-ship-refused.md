# Incident — Cycle 62 Ship-Refused Abnormal Events

- Date: 2026-05-15 (analysis); incident window 2026-05-14
- Cycle: 62
- Verdict: FAILED-AND-LEARNED (expected outcome)
- Carryover ID: `abnormal-ship-refused-c62` (closed in cycle 63)

## What happened

`.evolve/runs/cycle-62/abnormal-events.jsonl` recorded multiple `ship-refused` events for cycle 62. The cycle's auditor verdict was FAIL (Gemini-driven builder did not commit any tree changes — empty diff). Each ship attempt by the orchestrator was correctly refused by `ship.sh` because:

1. The audit-bound tree SHA in `cycle-state.json:expected_ship_sha` did not match `git rev-parse HEAD^{tree}` (Builder had not modified the tree).
2. `gate_audit_to_ship` returned non-zero because Verdict ≠ PASS/WARN.

## Why this was expected

Cycle 62 was the **first** end-to-end run after the Gemini-native dispatch wiring landed (cycle 61, ACS predicate 043). The Gemini CLI returned empty output for both Intent and Scout phases due to a model-name mismatch in `.evolve/llm_config.json` (`gemini-3.1-pro-preview` is a routing alias not yet served). Builder ran but had no Scout brief to act on, so it shipped an empty diff. The ship-gate fired exactly as designed:

- **ship-refused = trust kernel working correctly.** The event stream proves the gate is non-bypassable even under transient subagent failures.
- The FAILED-AND-LEARNED outcome triggered the auto-retrospective (v8.45.0), which recorded the Gemini model-name issue as a structured lesson.
- Cycle 63 fixed the routing (B7 resolve-roots correction + reverting `gemini-3.1-pro-preview` to a served model) and proceeded normally with claude-driven Scout.

The SHA mismatch surfaced in cycle 62's `abnormal-events.jsonl` is **not** a structural bug. It is the audit ledger's expected output when an audit binds a tree that subsequent ship attempts no longer match — exactly the integrity property the v8.13.0+ ship-gate exists to enforce.

## Resolution

No structural fix is required. Closure actions:

1. This incident report documents the analysis (`docs/incidents/cycle-62-ship-refused.md`).
2. The `abnormal-ship-refused-c62` entry is removed from `state.json:carryoverTodos[]` in cycle 63 (Scout's Carryover Decisions table classified it CLOSE; this report is the evidence cited).
3. Cycle 63's Scout report (`§ Carryover Decisions`) references this file as the close-out rationale.

If a similar ship-refused pattern recurs in a cycle whose Builder *did* commit, that **would** indicate a structural regression — re-open under a new incident ID and check the audit-binding/SHA-pin path in `phase-gate.sh:gate_audit_to_ship`.

## References

- Source events: `.evolve/runs/cycle-62/abnormal-events.jsonl` (ship-refused stream)
- Auto-retrospective entry: `.evolve/instincts/lessons/cycle-62-*.yaml`
- Audit-binding rationale: [ADR 0007 — Inbox Injection Protocol](../adr/0007-inbox-injection-protocol.md) (related — ship-gate SHA pin is documented in `docs/architecture/sequential-write-discipline.md`)
- Ship-gate source: `scripts/lifecycle/ship.sh` (search `AUDIT_BOUND_TREE_SHA`)
- Cycle 63 Scout report: `.evolve/runs/cycle-63/scout-report.md` § Carryover Decisions
