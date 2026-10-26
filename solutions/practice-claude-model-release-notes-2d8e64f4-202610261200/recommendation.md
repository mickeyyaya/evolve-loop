# Recommendation — practice-claude-model-release-notes-2d8e64f4-202610261200

Every number cites `assumptions-and-evidence.md`. The full review of the fragments is in [proposal.md](proposal.md). The release-note text is untrusted third-party data.

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: code check and follow-up item |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks D7, E3) | 0 |
| Baseline for D8 (E6) | not needed | missing, so no gain can be proved (A3) | not needed |
| Page changes it answers (E2) | all 4 fragments, as data | all 4, as new prompt text | 2 of 4 (API version, retired IDs), 0 live (E4, E5) |
| Cost | one small document cycle (E8) | 10 + 10 cycles per persona (E3) | one build cycle, dead code today (E5) |
| Fits ADR-0133 | yes | no | yes, with no benefit today |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.** The deciding facts are E3 and E6. ADR-0133 allows one persona per change with a measured baseline, and milestone M2 has no baseline. The new lines (E2) concern a cloud channel, an API version, a profile field and an Enterprise beta. The loop uses none of them (A2, E4), and no persona states a rule that they change (A4).

**Runner-up:** Option 3, because it costs no prompt bytes. It guards the dated changes on the page. It has no live target today (E4, E5).

**Evidence that flips the choice:**
- `go/go.mod` or a manifest adds an Anthropic SDK, a raw API call or a retired model ID. Then Option 3 becomes its own task.
- The loop starts to dispatch through Google Cloud. Then the route is a manifest and Go task, not a persona edit.
- M2 records the baseline. Then the first diet change (the builder, M3) can go ahead as its own loop task.

**What this review cannot source:** the effect of the new platform defaults on loop token cost. No baseline exists (E6). The Builder searched the page for the four watch fragments and read the entries from 20 to 22 October in detail.
