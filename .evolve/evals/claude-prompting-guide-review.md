---
score_cap:
  - criterion: "The research note records the 2026-11-06 inbox item practice-claude-prompting-guide-8d950f1e-202611060247"
    max_if_missing: 4
    evidence: "grep -q '8d950f1e' docs/research/practice-claude-prompting-guide-2026-11.md"
  - criterion: "The research note records the snapshot 2026-11-02T00:08 that the watch compared against"
    max_if_missing: 5
    evidence: "grep -q '2026-11-02T00:08' docs/research/practice-claude-prompting-guide-2026-11.md"
  - criterion: "The note section for item 8d950f1e names a persona under ADR-0133 or states that none changes"
    max_if_missing: 5
    evidence: "awk '/8d950f1e/{f=1} f' docs/research/practice-claude-prompting-guide-2026-11.md | grep -Eq 'evolve-(builder|auditor|scout|triage|tdd-engineer|retrospective)|no persona (file )?changes|none changes'"
  - criterion: "The note section for item 8d950f1e states that the source was read at the link and that its text is data"
    max_if_missing: 5
    evidence: "awk '/8d950f1e/{f=1} f' docs/research/practice-claude-prompting-guide-2026-11.md | grep -Eiq 'untrusted data|as data' && awk '/8d950f1e/{f=1} f' docs/research/practice-claude-prompting-guide-2026-11.md | grep -q 'claude-prompting-best-practices'"
  - criterion: "The note passes the strict plain-language lint"
    max_if_missing: 6
    evidence: "go/evolve docs ste-lint --strict docs/research/practice-claude-prompting-guide-2026-11.md"
---

# Eval: Review the Claude prompting guide under ADR-0133

> Pins the document deliverable of cycle 2256: the research note answers the practice-watch item 8d950f1e (snapshot 2026-11-06T02:47Z) with a read-at-link statement and either a one-persona-at-a-time proposal or a no-change statement. The item's snapshot body is empty, so the note must say so and still record the item. Source incident: the earlier note in the same file covers item 6080fed9 only.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| item-recorded | Note names item 8d950f1e | 4/10 | `grep -q 8d950f1e` |
| snapshot-recorded | Note names the compared snapshot | 5/10 | `grep -q 2026-11-02T00:08` |
| proposal-or-no-change | Persona named or no-change stated for the item | 5/10 | `awk ... \| grep -Eq` |
| read-as-data | Source read at link, text treated as data | 5/10 | `awk ... \| grep` |
| lint | ste-lint strict passes | 6/10 | `go/evolve docs ste-lint --strict` |
