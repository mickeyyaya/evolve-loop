# Recommendation — practice-agy-cli-changelog-page-e7652e1f-202610261200

Every number cites [assumptions-and-evidence.md](assumptions-and-evidence.md).
The line-by-line review is in [proposal.md](proposal.md).

## Options Compared

| Criterion | Option 1: no persona change, record verdict | Option 2: edit personas now | Option 3: driver config check and follow-up item |
|---|---|---|---|
| Persona edits | 0 | 2 or more (breaks D7, E5) | 0 |
| Baseline for D8 (E6) | not needed | missing, so no gain can be proved | not needed |
| Releases evaluated | v2.26.0 (E2) and v2.25.0 (E3) | v2.26.0 and v2.25.0 | v2.26.0 and v2.25.0 |
| Cost | one small document cycle (E10) | 10 + 10 cycles per persona (E5) | one build cycle |
| Fits ADR-0133 | yes | no | yes, but outside persona scope |

## Recommendation

**Adopt Option 1: no persona changes in this cycle.**
The deciding facts are E5 and E6.
ADR-0133 permits one persona change at a time with a measured baseline.
Milestone M2 has not recorded that baseline.
The v2.26.0 (E2) and v2.25.0 (E3) updates improve desktop UI and runtime stability.
No line alters a rule that a persona states (A4).

**Runner-up:** Option 3.
It costs zero prompt bytes and targets the string patterns in the agy manifest (E9).
Keep Option 3 until an upstream release alters the prompt footer or quota text.

**Evidence that flips the choice:**
- An upstream release changes the `? for shortcuts` footer or quota limit text (E9). Then Option 3 becomes necessary.
- A release changes headless `-p` execution or CLI exit codes in a way that affects phase instructions (E4).
- Milestone M2 completes baseline data collection. Then the builder diet change (M3) can proceed under its own cycle.

**What this review cannot source:**
The exact latency effect of upstream socket auto-retry on loop test lanes.
No telemetry baseline exists yet (E6).
The review did not execute the desktop UI features (A2).
