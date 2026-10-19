# Recommendation — practice-agy-cli-changelog-page-b5729420-202610192320

Every number cites `assumptions-and-evidence.md`. The line-by-line review is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: driver config check and follow-up item |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks D7, E8) | 0 |
| Baseline for D8 (E9) | not needed | missing, so no gain can be proved (A3) | not needed |
| Run-behavior lines of E5 that it answers | all, as data | all, as new prompt text | 2 text matches (E10) |
| Cost | one small document cycle (E9) | 10 + 10 cycles per persona (E8) | one build cycle (E10) |
| Fits ADR-0133 | yes | no | yes, but it is not a persona matter |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.** The deciding facts are E8 and E9. ADR-0133 allows one persona per change with a measured baseline, and milestone M2 has not recorded that baseline. The run-behavior lines (E5) are fixes the CLI owns, and no persona states a rule they change (A4).

**Runner-up:** Option 3. It costs no prompt bytes and it targets the only two text matches in the agy manifest (E10). Hold it until a watch diff shows a reworded footer or limit message.

**Evidence that flips the choice:**
- A later release rewords the usage-limit message or the `? for shortcuts` footer (E10). Then Option 3 gets a live target.
- A release changes headless `-p` subagent questions, the stall limit or the exit codes in a way a phase prompt states a rule about (E5).
- M2 records the baseline. Then the first diet change (the builder, M3) can go ahead as its own loop task.

**What this review cannot source:** the effect of the v1.3.6 stall retry on loop retries. No baseline exists (E9). The review also did not read the output of the `agy changelog` command, so the two sources may differ (A1).
