# Recommendation — practice-claude-model-release-notes-e9fe428e-202611021720

Every number cites `assumptions-and-evidence.md`. The full review of the fragments is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: setting check and follow-up item |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks D7, E3) | 0 |
| Baseline for D8 (E6) | not needed | missing, so no gain can be proved (A3) | not needed |
| Page changes it answers (E2) | all, as data | all, as new prompt text | 2 of 6 (fast mode, Opus 5.5 thinking), 0 live (E5) |
| Cost | one small document cycle (E8) | 10 + 10 cycles per persona (E3) | one build cycle, dead code today (E5) |
| Fits ADR-0133 | yes | no | yes, with no benefit today |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.** The deciding facts are E3 and E6. ADR-0133 allows one persona per change with a measured baseline, and milestone M2 has no baseline. The lines (E2) concern API fields, an analytics endpoint and a size limit. The loop sends none of them (A2, E4, E5), and no persona states a rule that they change (A4).

**Runner-up:** Option 3, because it costs no prompt bytes. It guards the one open dated change: fast mode for Claude Opus 4.8 ends on 2026-11-30. It has no live target today (E5).

**Evidence that flips the choice:**
- A manifest or profile sends `speed: "fast"` to `claude-opus-4-8`, or sets a thinking field on Opus 5.5. Then Option 3 becomes a fix to the manifest as its own task.
- The loop starts to call the API directly. Then the 128 MB limit and the analytics endpoints become a code path, not a persona matter.
- M2 records the baseline. Then the first diet change (the builder, M3) can go ahead as its own loop task.

**What this review cannot source:** the effect of the new price and defaults on loop token cost. No baseline exists (E6). The Builder read the entries from 22 September to 9 November in detail. It searched the older text only for the watch fragments.
