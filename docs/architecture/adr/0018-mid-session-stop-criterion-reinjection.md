# ADR-0018 — Mid-Session STOP CRITERION Re-Injection

| Field | Value |
|-------|-------|
| Status | Proposed |
| Date | 2026-05-19 |
| Cycle (research) | 87 |
| Cycle (implementation) | 88 |
| Author | Scout cycle-87 via `knowledge-base/research/agentic-pipeline-enforcement-2026.md` |

---

## Status

**Proposed** — implementation slotted for cycle-88. This ADR is a research-to-action bridge; no code changes in cycle-87.

---

## Context

Cycle-86 produced a `turn-overrun` abnormal event: the intent agent ran 12 turns against a ceiling of 10. Research deposited in cycle-87 (`knowledge-base/research/agentic-pipeline-enforcement-2026.md`, Finding 3) identifies the root cause as **instruction adherence attenuation**:

> "A significant within-session attenuation effect where LLM agent compliance decreases by approximately 5.6% per generated function." — ACM AISystems 2024 (https://dl.acm.org/doi/full/10.1145/3703412.3703439)

At 57 turns (cycle-86 Builder), this produces ~30–60% degradation in adherence to the STOP CRITERION injected in the system prompt. The current pipeline has no mechanism to counteract this effect — the turn ceiling is a bare integer in `--max-turns`, and when exceeded, the run fails cold with no recovery signal.

---

## Decision

For agents with `max_turns > 8`, inject a compressed re-statement of the STOP CRITERION at turn `ceil(max_turns * 0.7)` as a mid-session anchor.

**Implementation target:** `scripts/dispatch/subagent-run.sh`

**Mechanism:** Turn-count-aware prompt re-injection strategy. At the 70% turn threshold, a one-paragraph STOP CRITERION summary (~100 tokens) is appended to the running prompt, counteracting the attenuation effect before it becomes critical. This converts the current "fail loudly at the ceiling" pattern into a "warn before the ceiling, enforce at the ceiling" approach.

**Constraints honored:**
- Bash 3.2 compatible (arithmetic only, no associative arrays, no `mapfile`).
- Additive change — no existing scripts modified beyond `subagent-run.sh`.
- Dual-root pattern: reads constraint definitions from `PLUGIN_ROOT`, injects into the claude invocation.
- No new agent profiles or scripts required.

---

## Consequences

**Positive:**
- Expected 30–50% reduction in `turn-overrun` abnormal events based on adherence attenuation curve.
- Mid-session re-injection is additive — existing turn ceiling enforcement is unchanged.
- ~20 lines of bash; low blast radius.

**Negative / risks:**
- Adds ~100 tokens per eligible agent invocation at the re-injection point.
- If the threshold is set too early (<70%), agents may receive the reminder prematurely; if too late (>80%), it may not help before the ceiling is hit.

**Deferred:**
- Finding 1 (AgentSpec rule-engine wrapper around `subagent-run.sh`) is orthogonal and higher blast radius; deferred to cycle-88 or later.
- Finding 2 (Refute-or-Promote CMC pre-step in Auditor phase) targets code-audit-fail rate; orthogonal to turn-overrun; deferred to cycle-88 or later.

---

## References

- Research dossier: `knowledge-base/research/agentic-pipeline-enforcement-2026.md` (Finding 3)
- Source paper: "Methodology for Quality Assurance Testing of LLM-based Multi-Agent Systems." ACM AISystems 2024. https://dl.acm.org/doi/full/10.1145/3703412.3703439
- Cycle-86 abnormal event evidence: `.evolve/runs/cycle-86/abnormal-events.jsonl` (`turn-overrun` entry)
