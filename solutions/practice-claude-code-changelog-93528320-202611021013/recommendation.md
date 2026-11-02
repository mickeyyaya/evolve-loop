# Recommendation — practice-claude-code-changelog-93528320-202611021013

Every number cites `assumptions-and-evidence.md`. The full review is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: guard for Opus 4.8 fast mode |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks D7, E2) | 0 |
| Baseline for D8 (E3) | not needed | missing, so no gain can be proved (A3) | not needed |
| Lines of 2.1.316 that it answers (E1) | all, as data | all, as new prompt text | 1 deprecation, 0 live (E4) |
| Cost | one small document cycle (E7) | 10 + 10 cycles per persona (E2) | one build cycle, dead code (E4) |
| Fits ADR-0133 | yes | no | yes, with no benefit |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.** The deciding facts are E2 and E3. ADR-0133 allows one persona per change with a measured baseline, and `evolve prompt stats` (M2) does not exist yet. The runtime lines (E1) are settings and fixes the CLI owns, and no persona states a rule they change (E4, A4).

**Runner-up:** Option 3, because it costs no prompt bytes. It has no live target today (E4).

**Evidence that flips the choice:**
- The loop routes a phase to Opus 4.8 in fast mode before 2026-11-30. Then Option 3 gets a live target.
- The loop adopts a subagent effort setting. Then the routing owner reviews the effort rule.
- M2 records the baseline. Then the first diet change (the builder, M3) can go ahead as its own loop task.

**What this review cannot source:** the effect of the 2.1.316 fixes on loop ship rate. No baseline exists (E3).
