# Recommendation — practice-claude-prompting-guide-f509428a-202610261159

Every number cites `assumptions-and-evidence.md`. The full review is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: deterministic guard and follow-up item |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks D7, E3) | 0 |
| Baseline for D8 (E4) | not needed | missing, so no gain can be proved (A3) | not needed |
| Prompting lines that it addresses (E2) | all, as untrusted data | all, as new prompt text | prefill only (E5) |
| Cost | one small document cycle (E9) | 10 + 10 cycles per persona (E3) | one build cycle, dead code (E5) |
| Fits ADR-0133 | yes | no | yes, with no benefit |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.** The deciding facts are E3 and E4. ADR-0133 allows one persona per change with a measured baseline. The command `evolve prompt stats` (milestone M2) does not exist yet. The prompting guide lines (E2) are untrusted third-party data, and no persona states a rule they change (E5, A4).

**Runner-up:** Option 3, because it adds no prompt bytes. It has no live target today (E5).

**Evidence that flips the choice:**
- Milestone M2 ships and records the 10-cycle baseline. Then the builder diet (milestone M3) can proceed as its own task.
- An upstream model change causes measured tool overtriggering across loop runs. Then Option 2 can be considered after M2.
- The loop introduces prefill requests that require runtime validation. Then Option 3 gains a live target.

**What this review cannot source:** the exact impact of calm phrasing on loop token counts. No baseline exists (E4).
