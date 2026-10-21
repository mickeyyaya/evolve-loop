# Recommendation — practice-claude-model-release-notes-bfd090d6-202610210011

Every number cites `assumptions-and-evidence.md`. The full review of the fragments is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: model ID check and follow-up item |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks D7, E3) | 0 |
| Baseline for D8 (E6) | not needed | missing, so no gain can be proved (A3) | not needed |
| Page changes it answers (E2) | all, as data | all, as new prompt text | 1 of 3 (the retirement), 0 live (E5) |
| Cost | one small document cycle (E8) | 10 + 10 cycles per persona (E3) | one build cycle, dead code today (E5) |
| Fits ADR-0133 | yes | no | yes, with no benefit today |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.** The deciding facts are E3 and E6. ADR-0133 allows one persona per change with a measured baseline, and milestone M2 has no baseline. The new lines (E2) concern an Enterprise organization setting and a retirement date. The loop sets no hook (A2, E4), names no retired model (E5), and no persona states a rule that they change (A4).

**Runner-up:** Option 3, because it costs no prompt bytes. It guards the one dated change on the page: Claude Sonnet 4.5 retires on 2026-11-30. It has no live target today (E5).

**Evidence that flips the choice:**
- A manifest, profile or policy file names `claude-sonnet-4-5-20250929` or another retired ID. Then Option 3 becomes a fix to the manifest as its own task.
- The loop starts to run under an Enterprise Inference hook. Then the hook verdict becomes a dispatch failure class, and a code path, not a persona, handles it.
- M2 records the baseline. Then the first diet change (the builder, M3) can go ahead as its own loop task.

**What this review cannot source:** the effect of the new platform defaults on loop token cost. No baseline exists (E6). The Builder read the entries down to 30 September in detail. It searched the older text only for the three watch fragments.
