# ADR-0015: Auditor Cold-Move — Stage 8

**Status:** Accepted
**Cycle:** 77
**Campaign:** Token Optimization (Cycles 74–77)
**Cross-links:** Stage 6 (cycle 75, commit 2bb8d90), Stage 7 (cycle 76, commit 4049cda)

## Context

The token-optimization campaign (Cycles 74–77) applies progressive disclosure to
evolve-loop agent personas. Hot-path guidance stays inline; cold-path reference
material (format specs, full tables, extended examples) moves to `*-reference.md`
companion docs loaded on-demand.

Stages 6 and 7 cold-moved the Orchestrator Phase Loop body (−55 lines) and
Builder Step 9 / POSTHOC / Skills sections (−54 lines) respectively. Stage 8
targets the Auditor persona (`agents/evolve-auditor.md`, baseline 333 lines).

The `## Output` section (lines 168–244, 77 lines) contains:
- A 68-line `workspace/audit-report.md` markdown template (full report structure)
- A JSON `Ledger Entry` template

This is pure format specification. The Auditor does not make decisions from this
template during turns 1–N; it only consults it once when writing the final report.
The pattern is structurally identical to Stage 7 (Builder cold-move) with one
divergence: the Auditor's Output section is a purer format template — no embedded
behavioral rules — making it a safer cold-move than Stage 7.

The Auditor already reads other reference doc sections on-demand
(`adaptive-strictness`, `review-checklist`, `egps-computation`, `handoff-json`).
Adding `output-template` follows the established convention.

## Decision

Cold-move `## Output` section (lines 168–244, 77 lines) from `agents/evolve-auditor.md`
to `agents/evolve-auditor-reference.md` section `output-template`.

Replace the removed section with a 3-line pointer:

```markdown
## Output

Read [agents/evolve-auditor-reference.md](agents/evolve-auditor-reference.md) section `output-template` for the full `workspace/audit-report.md` format and `Ledger Entry` JSON template.
```

## Auditor-vs-Builder Divergences

| Aspect | Builder Stage 7 | Auditor Stage 8 |
|---|---|---|
| Moved content | POSTHOC examples + Skills list (behavioral reference) | audit-report.md template + Ledger Entry (format spec only) |
| Behavioral rules embedded | Yes (POSTHOC rules included within the section) | No — pure format template |
| Cold-move safety | Moderate (behavioral ref sections need careful annotation) | High (format-only, no behavioral content) |
| Reference doc size before | ~95 lines | 95 lines |
| Reference doc size after | ~172 lines | ~172 lines |

## Consequences

| Metric | Before | After | Delta |
|---|---|---|---|
| `agents/evolve-auditor.md` lines | 333 | ~259 | −74 (−22.2%) |
| `agents/evolve-auditor-reference.md` lines | 95 | ~172 | +77 |
| Hot-path preserved | — | — | Verdict Rules, POSTHOC enforcement, Constitutional checklist, Hypothesis falsification, WARN-elevation hardening, STOP CRITERION all remain inline |
| Pointer overhead | — | 3 lines | Replaces 77-line template |

The ≥10% reduction target is exceeded (22.2% actual). The reference doc grows to
~172 lines — well below a bottleneck threshold.

**Behavioral impact:** Zero. The Auditor's decision-making flow is unchanged.
The Output template is consulted only at write-time (after verdict is decided),
so deferring it to on-demand load does not affect audit quality.
