# ADR 0016 — Retrospective Cold-Move Stage 9

- Status: Accepted
- Date: 2026-05-18
- Cycle: 78
- Predecessor: [ADR 0015 — Auditor Cold-Move Stage 8](0015-auditor-cold-move-stage8.md)
- Supersedes: inline reference content in `agents/evolve-retrospective.md`

## Context

`agents/evolve-retrospective.md` stood at 281 lines with no reference doc.
Two sections were classified as cold (on-demand lookup only):

1. **`## Structured Output: handoff-retrospective.json (C3)`** (21 lines) — a
   schema table for the handoff JSON. Step 6 in the hot prompt already provides
   the concrete JSON example; the field-by-field type table is only consulted
   when a field value is ambiguous.

2. **Digest format block in `### 8. Write the digest`** (22 lines) — the
   markdown code-block template for `lessons-digest.md`. The imperative
   ("write a ≤500 token summary") stays hot; the exact format template is
   on-demand.

3. **`## Reference Index (Layer 3, on-demand)`** (5 lines) — already
   self-described as Layer 3; relocated to a consolidated reference index.

## Decision

Move the three cold sections to a new `agents/evolve-retrospective-reference.md`
with anchored sections matching the pattern established in Stage 3–8. Replace
each removed block in the hot prompt with a single pointer line.

Also raise `scout.json:max_turns` from 15 → 30 as a calibration free-rider
(three consecutive overruns falsify the prior ceiling; P75 × 1.2 = 39,
raising to 30 is a conservative middle ground).

## Consequences

| Metric | Before | After |
|---|---|---|
| `evolve-retrospective.md` lines | 281 | 241 (−14.2%) |
| Reference doc | none | `agents/evolve-retrospective-reference.md` (67 lines) |
| Scout `max_turns` | 15 | 30 |
| Behavioral change | none | none — pure structural split |

Hot-path content preserved: Steps 1–7 imperatives, Step 6 handoff JSON
example, all behavioral rules (Core Principles §1–4, Process §1–8, Out of
scope, Final checks).
