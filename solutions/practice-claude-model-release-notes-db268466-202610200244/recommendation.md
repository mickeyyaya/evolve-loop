# Recommendation — practice-claude-model-release-notes-db268466-202610200244

Every number cites `assumptions-and-evidence.md`. The full review of the fragments is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: deterministic guard and follow-up item |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks D7, E3) | 0 |
| Baseline for D8 (E5) | not needed | missing, so no gain can be proved (A3) | not needed |
| Page changes it answers (E2) | all, as data | all, as new prompt text | 0 live (E4) |
| Cost | one small document cycle (E9) | 10 + 10 cycles per persona (E3) | one build cycle, dead code (E4) |
| Fits ADR-0133 | yes | no | yes, with no benefit |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.** The deciding facts are E3 and E5. ADR-0133 allows one persona per change with a measured baseline, and milestone M2 has no baseline. The page changes (E2) concern API request fields and Models API output. The loop sends no such request (A2, E4), and no persona states a rule they change (E8, A4).

**Runner-up:** Option 3, because it costs no prompt bytes. It has no live target today (E4).

**Evidence that flips the choice:**
- The loop starts to call `GET /v1/models` or to send a `thinking` field. Then the catalog or the bridge gets a code change as its own task.
- A CLI release exposes a `thinking` setting that a phase prompt states a rule about (E6, H1).
- M2 records the baseline. Then the first diet change (the builder, M3) can go ahead as its own loop task.

**What this review cannot source:** the effect of the adaptive thinking default on loop token cost. No baseline exists (E5). Also unread: the last 27195 characters of the fetched page, which hold older entries than the ones above.
