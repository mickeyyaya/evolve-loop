# ADR-0017: Scout STOP CRITERION Densification (Cycle 78)

**Status:** Accepted  
**Date:** 2026-05-18  
**Cycle:** 78  
**Author:** Builder (Stage 10 token-optimization campaign)

---

## Context

`agents/evolve-scout.md` STOP CRITERION section accumulated to 45 lines across cycles 39–78 as each turn-overrun incident added new subsections. The section had three recurrence violations in cycles 69–78 (cycle 78 being the third). Each violation added guards without removing or compressing the previous guards — growing the section while degrading its scannability.

Root cause: verbose subsection headers + narrative prose made the actual halting rules hard to find at a glance. Scout accumulated turns partly because the stop criterion itself was buried in structure.

Prior stop criterion work: ADR-0010 (scout-stop-criterion-tightening). This ADR supersedes the prose strategy from ADR-0010 with a step-rewrite approach.

---

## Decision

Rewrite the `## STOP CRITERION` section in `agents/evolve-scout.md` to:

1. **≤20 lines** — scannable in one read
2. **Gate-numbered table** — all 5 gates explicit with `# | Gate | Satisfied when` columns
3. **Imperative, not narrative** — deadlines as inline bold statements, not headers + paragraphs
4. **No behavioral deletions** — all 5 turn deadlines (5, 7, 10), web cap (3), all 5 gate names, exit protocol, and banned-pattern list preserved

---

## Before / After

**Before (45 lines, 4 subsection headers):**
- `### Web Research Deadline (turn 5)` — 3 lines prose
- `### Emergency Exit (turn 7+)` — 4 lines prose  
- `### Web Search Cap` — 3 lines prose
- `### Completion Gates` — 7 lines (table, but said "four" gates despite having 5 rows)
- `### Exit Protocol` — 4 lines (said "all four gates" — stale count)
- `### Banned Post-Report Patterns` — 8 lines bulleted list

**After (20 lines, 1 subsection header):**
- 1-line halt condition statement
- 1-line deadlines (turns 5/7/10) + web cap — inline bold
- `### Gates (all five required)` — numbered table, 5 rows
- 1-line exit protocol
- 1-line banned-patterns with rationale pointer

**Stale-logic fix:** "all four gates" → "all five gates" (gate 5 `research-cache-section` was added in a later cycle without updating the prose count).

---

## Behavioral Guarantees Preserved

| Guarantee | Before location | After location |
|-----------|----------------|---------------|
| Turn 5 web deadline | `### Web Research Deadline (turn 5)` | **Deadlines** inline line |
| Turn 7 emergency exit + `TIME-BOUNDED` prefix | `### Emergency Exit (turn 7+)` | **Deadlines** inline line |
| Turn 10 hard stop | `### Emergency Exit (turn 7+)` | **Deadlines** inline line |
| 3 WebSearch/WebFetch cap | `### Web Search Cap` | **Deadlines** inline line |
| Gate 1: `system-health-complete` | Completion Gates table | Gates table row 1 |
| Gate 2: `inbox-audit-complete` | Completion Gates table | Gates table row 2 |
| Gate 3: `backlog-complete` | Completion Gates table | Gates table row 3 |
| Gate 4: `build-plan-written` | Completion Gates table | Gates table row 4 |
| Gate 5: `research-cache-section` | Completion Gates table | Gates table row 5 |
| One-write exit protocol | `### Exit Protocol` | **Exit** inline line |
| Banned post-report patterns | `### Banned Post-Report Patterns` | **Banned post-report** inline line |
| Cycle-39 rationale | Rationale paragraph | Inline parenthetical |

---

## Step-Rewrite Pattern

This rewrite follows the step-rewrite pattern established in Stage 7 (ADR-0014, commit 4049cda) and Stage 8 (ADR-0015, commit 709ba8f):

1. Identify sections with subsection headers wrapping ≤3 lines of content
2. Collapse subsection header + body into a single bold inline statement
3. Convert bullet lists to inline comma-separated lists when ≤5 items
4. Preserve gate names verbatim (grep-target invariant)
5. Fix stale counts (prose saying "four" when table had five rows)

---

## Consequences

- STOP CRITERION section: 45L → 20L (−55%)
- Total `agents/evolve-scout.md`: 259L → 232L (−10.4%)
- All 5 gate names remain grep-verifiable
- Turn-7/10 emergency exit triggers unchanged
- ADR-0010 strategy superseded; this ADR is the canonical record for cycle 78+

---

## Cross-References

- ADR-0010: `docs/architecture/adr/0010-scout-stop-criterion-tightening.md` (prior attempt, prose strategy)
- Stage 7 builder cold-move: commit 4049cda
- Stage 8 auditor cold-move: commit 709ba8f
- Stage 9 retrospective cold-move: commit 7dab664 (ADR-0016)
- Scout turn-overrun cycle 78: `agents/evolve-scout.md` third recurrence trigger
