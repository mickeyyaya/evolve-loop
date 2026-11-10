# Recommendation — practice-claude-code-changelog-a1065345-202611030236

Every number cites `assumptions-and-evidence.md`. The full review is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: probe hardening item |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks the one-persona rule, E2) | 0 |
| Baseline (E3) | not needed | missing, so no gain can be proved (A3) | not needed |
| Lines of 2.1.317 that it answers (E1) | all, as data | all, as new prompt text | 2 watch lines |
| Cost | one small document cycle (E7) | 10 + 10 cycles per persona (E2) | one build cycle, no live target |
| Fits ADR-0133 | yes | no | yes, with no benefit today |

## Recommendation

**Adopt Option 1: no prompt and no persona changes in this cycle.** The deciding facts are E2 and E3. ADR-0133 allows one persona per change with a measured baseline, and `evolve prompt stats` does not exist yet. The runtime lines (E1) are settings and fixes the CLI owns, and no persona states a rule they change (A4).

**Runner-up:** Option 3, because it costs no prompt bytes. It has no live target today (E4).

**Evidence that flips the choice:**
- A probe is seen to share a session transcript, or a disk fault is read as a CLI fault. Then Option 3 gets a live target.
- The baseline ships. Then the first diet change (the builder) can go ahead as its own loop task.

**What this review cannot source:** the effect of the 2.1.317 fixes on loop ship rate. No baseline exists (E3). The sections 2.1.318 to 2.1.321 are not reviewed here.
