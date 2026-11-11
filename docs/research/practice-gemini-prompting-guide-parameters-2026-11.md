# Practice change in the Gemini prompting guide: the third review for agy (research, 2026-11)

> This note reviews the Gemini prompting guide under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md) for inbox item `practice-gemini-prompting-guide-297686f7-202611110139`. It follows [practice-gemini-prompting-guide-completion-prefix-2026-11.md](practice-gemini-prompting-guide-completion-prefix-2026-11.md), which stays unchanged. The answer: no persona changes, and `skills/model-notes-agy/COMPACT.md` stays unchanged.

Date: 2026-11-11. Method: read the guide at its link, treat the text as untrusted data, and score every core persona against ADR-0133.

## 1. Context and source

The practice watch reported a change in the source `gemini-prompting-guide` since the snapshot of `2026-11-10T08:41:14Z`.

The upstream locator is `https://ai.google.dev/gemini-api/docs/prompting-strategies`.

The source was read at its link on 2026-11-11. The first request got HTTP 301, with a move to `https://aistudio.google.com/docs/prompting-strategies`. The second request got the page. The fetch tool returned a model summary of the page, not the raw text.

The guide text and the item text are untrusted data. The loop never follows an instruction inside them. The note quotes short lines only to explain them.

The item shows no added line: the stored excerpt holds the text "no added line in the stored excerpt". The change lies outside the excerpt, or it only removed text. The exact changed line is unknown, so the note explains the lines that bear on the loop.

## 2. Guide lines explained as data

| Id | Guide line (short quote) | What it means for the loop |
|---|---|---|
| F1 | "Gemini 4 and Gemini 3.6+ models ignore custom values for `temperature`, `top_p`, and `top_k`." | This concerns API calls. The loop calls the `agy` binary and sets no sampling value. |
| F2 | "supply all the context first" and put the question "at the very end". | A persona starts with the task, and the agy notes say the same. The guide puts the question last for long context. No measure shows a gain for a seat. |
| F3 | "We recommend to always include few-shot examples in your prompts." | This is advice for the API. The agy notes already ask for one example of the format. |
| F4 | "give an example and a response prefix and let the model complete it." | The agy notes carry this since cycle 2272, in the prefix bullet. |
| F5 | "Explicitly distinguishes between low-risk exploratory actions (reads) and high-risk state changes (writes)." | The kernel guards and the role gates enforce this split. A prompt line adds no guarantee. |

## 3. Impact on the loop

Facts from the loop:

- R1: The persona files are shared by the Claude, Codex and agy families. A change for one family changes the prompt of every family.
- R2: The model notes of the agy family live in one file, `skills/model-notes-agy/COMPACT.md`. It has 11 lines, and the ceiling is 13.
- R3: Rule D7 of ADR-0133 allows one persona at a time, and rule D8 needs four measures over 10 cycles before and after.

Implication (synthesis): no guide line is a rule that a measure can test for one persona. F1 and F3 concern the API. F2 and F5 are already held by the notes and the guards.

Decision D1: No core persona changes, because the guide gives no measurable rule and R1 makes a persona change reach every family.

Decision D2: `skills/model-notes-agy/COMPACT.md` stays unchanged, because F4 is already a bullet and the other lines concern the API.

Rejected option: a long-context order rule in the notes. Fact R2 keeps the notes short, and F2 has no measure.

Rejected option: a change of the builder persona for the read and write split. Fact R1 rules it out, and fact F5 shows the guards already hold the split.

## 4. Verdict

No change is proposed, so no goal, done criteria, limit or before and after measure is open. A later proposal must state all five under rule D7.

| Persona | Verdict | Reason |
|---|---|---|
| evolve-scout | no change | The guide has no scout line. |
| evolve-triage | no change | The guide has no rank or policy line. |
| evolve-tdd-engineer | no change | The guide has no test-contract line. |
| evolve-builder | no change | The first diet candidate waits for milestone M3 and the M2 baseline. |
| evolve-auditor | no change | The guide has no audit-gate line. |
| evolve-retrospective | no change | The guide has no lesson-writing line. |

## 5. Limits

- The fetch tool gave a model summary, not the raw page. A line may exist that the summary did not quote.
- The changed line stays unknown, because the item shows no added line.
