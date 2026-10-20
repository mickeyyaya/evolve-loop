# Recommendation — practice-agy-cli-changelog-command-be72426b-202610200639

Every number cites `assumptions-and-evidence.md`. The line-by-line review is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: driver config check and follow-up item |
|---|---|---|---|
| Persona edits | 0 | 2 (breaks D7, E6) | 0 |
| Baseline for D8 (E7) | not needed | missing, so no gain can be proved (A4) | not needed |
| Run-behavior lines of E3 that it answers | both, as data | both, as new prompt text | 0 (it targets E4 text matches) |
| Cost | one small document cycle | 10 + 10 cycles per persona (E6) | one build cycle |
| Fits ADR-0133 | yes | no | yes, but it is not a persona matter |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.** The deciding facts are E6 and E7. ADR-0133 allows one persona per change with a measured baseline, and milestone M2 has not recorded that baseline. The two run-behavior lines (E3) are fixes the CLI owns, and no persona states a rule they change (A4).

**Runner-up:** Option 3. It costs no prompt bytes and it targets the text matches in the agy manifest (E4). Its targets are the footer and limit text (E4) and the `agy plugin validate` and `agy plugin install` calls of the skills-publish command (E8). Hold it until a watch diff shows a reworded footer or limit message.

**Evidence that flips the choice:**
- A later release rewords the usage-limit message or the `? for shortcuts` footer (E4). Then Option 3 gets a live target.
- A later release changes the output or flags of `agy plugin validate` or `agy plugin install` (E8). Then Option 3 also covers the skills-publish command.
- A phase prompt starts to tell a seat to run subagents. Then the compaction line (E3) matters to that persona.
- M2 records the baseline (E7). Then the first diet change (the builder, M3) can go ahead as its own loop task.

**What this review cannot source:** the effect of the compaction fix on long agy seats. No baseline exists (E7). The text of `agy changelog` may also differ from the web page that the earlier item read (A1).
