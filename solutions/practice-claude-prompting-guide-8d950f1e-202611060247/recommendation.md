# Recommendation — practice-claude-prompting-guide-8d950f1e-202611060247

Every number cites `assumptions-and-evidence.md`. The full review is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no change, record verdict | Option 2: soften builder now | Option 3: marker check and follow-up |
|---|---|---|---|
| Persona edits | 0 | 1 | 0 |
| Baseline for D8 (E3) | not needed | missing (A2) | not needed |
| Guide lines it answers (E2) | all, as data | one persona only | 0 directly |
| Cost (E6) | one small document cycle | 10 + 10 cycles | two build cycles |
| Fits ADR-0133 | yes | no | yes, with later benefit |

## Recommendation

**Adopt Option 1: no persona file changes in this cycle.** The deciding facts are E3 and A2. ADR-0133 allows one persona per change with a measured baseline, and none exists. The proposal of E5 stays: builder first, then auditor, then scout.

**Runner-up:** Option 3, because it gives the before measure at low prompt cost.

**Evidence that flips the choice:**
- M2 records the baseline. Then the builder diet can start as its own loop task (E3).
- A later read of the guide shows a new rule that a persona contradicts (A1).

**What this review cannot source:** the effect of softer wording on ship rate. No baseline exists (A2). The page shows no update date, so the changed line is unknown (E1).
