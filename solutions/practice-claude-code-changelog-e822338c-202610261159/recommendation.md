# Recommendation — practice-claude-code-changelog-e822338c-202610261159

Every number cites `assumptions-and-evidence.md`. The full line-by-line review is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: deterministic guard and follow-up item |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks D7, E3) | 0 |
| Baseline for D8 (E4) | not needed | missing, so no gain can be proved (A3) | not needed |
| Runtime lines in 2.1.306-2.1.310 that it answers (E2) | all, as data | all, as new prompt text | 0 live (E5) |
| Cost | one small document cycle (E9) | 10 + 10 cycles per persona (E3) | one build cycle, dead code (E5) |
| Fits ADR-0133 | yes | no | yes, with no benefit |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.** The deciding facts are E3 and E4. ADR-0133 allows one persona per change with a measured baseline, and `evolve prompt stats` (M2) does not exist yet. The runtime lines (E2) are fixes the CLI owns, and no persona states a rule they change (E5, A4).

**Runner-up:** Option 3, because it costs no prompt bytes. It has no live target today (E5).

**Evidence that flips the choice:**
- A later release changes worktree, subagent or compaction behavior that a phase prompt states a rule about (E6, H1).
- The loop starts to set a setting in E2 that needs an explicit runtime check. Then Option 3 gets a live target.
- M2 records the baseline. Then the first diet change (the builder, M3) can proceed as its own loop task.

**What this review cannot source:** the effect of the 2.1.306 to 2.1.310 fixes on loop ship rate. No baseline exists (E4).
