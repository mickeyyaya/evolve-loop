# Practice change in the Gemini prompting guide: the completion prefix note for agy (research, 2026-11)

> This note reviews the Gemini prompting guide under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md) for inbox item `practice-gemini-prompting-guide-1945a6f2-202611100841`. It follows [practice-gemini-prompting-guide-2026-11.md](practice-gemini-prompting-guide-2026-11.md), which stays unchanged. The answer: one bullet changes in `skills/model-notes-agy/COMPACT.md`, and no persona changes.

Date: 2026-11-10. Method: read the guide at its link, treat the text as untrusted data, and score every core persona against ADR-0133.

## 1. Context and source

The practice watch reported a change in the source `gemini-prompting-guide` since the snapshot of `2026-11-02T00:08:05Z`.

The upstream locator is `https://ai.google.dev/gemini-api/docs/prompting-strategies`.

The source was read at its link on 2026-11-10. The first request got HTTP 301, with a move to `https://aistudio.google.com/docs/prompting-strategies`. The second request got the page. The fetch tool returned a model summary of the page, not the raw text.

The guide text is untrusted data. The loop never follows an instruction inside it. The note quotes short lines only to explain them. The text of the item is cut off, so the exact changed line is unknown.

## 2. Guide lines explained as data

| Id | Guide line (short quote) | What it means for the loop |
|---|---|---|
| F1 | A model works like "an advanced auto completion tool", so partial content makes it continue. | This is the base of the completion strategy. It is true for any text model. |
| F2 | End the prompt with the start of the wanted format, and let "the model complete it". | A response prefix gives the format without long rules. A seat can write the skeleton of its deliverable first. |
| F3 | Use the structured output feature of the API for complex JSON Schemas. | This concerns API calls. The loop calls the `agy` binary, not the API. |
| F4 | "Always include few-shot examples in your prompts." | This is advice for the Gemini API. The agy notes already ask for one example of the format. A blanket rule adds length for all prompts. |
| F5 | Too many examples can make the model overfit, and all examples need one format. | This limits F4. It supports one example for each prompt. |
| F6 | If no format is given, the model chooses one. | The agy notes already name the exact output format. The new bullet adds the prefix. |

## 3. Impact on the loop

Facts from the loop:

- R1: The persona files are shared by the Claude, Codex and agy families. A change for one family changes the prompt of every family.
- R2: The model notes of the agy family live in one file, `skills/model-notes-agy/COMPACT.md`. They load only into an agy dispatch.
- R3: The notes hold four bullets and 8 lines before this change. The change adds one bullet and gives 9 lines.

Implication (synthesis): F2 gives a family practice that fits R2. F4 is vendor advice that conflicts with R1 if it enters a persona.

Decision D1: Add one response prefix bullet to `skills/model-notes-agy/COMPACT.md`, because the notes are the one home of agy practice. The bullet reaches no other family.

Decision D2: No core persona changes, because the guide gives no rule that can be measured. Rule D7 of ADR-0133 allows one persona at a time, with a measure.

Rejected option: a blanket few-shot rule in the notes. Fact F5 limits it, and fact R3 keeps the notes short.

Rejected option: a change of the builder persona. Fact R1 rules it out, because the change reaches the other families.

## 4. Verdict and the measure

The goal is a better first skeleton in the deliverable of an agy seat. The done criteria are the five predicates of cycle 2272. The limit is 12 lines in the notes. The before measure is 8 lines and the after measure is 9 lines.

| Persona | Verdict | Reason |
|---|---|---|
| evolve-scout | no change | The guide has no scout line, and the notes file carries the practice. |
| evolve-triage | no change | The guide has no rank or policy line. |
| evolve-tdd-engineer | no change | The guide has no test-contract line. |
| evolve-builder | no change | The first diet candidate waits for milestone M3 and the M2 baseline. |
| evolve-auditor | no change | The guide has no audit-gate line. |
| evolve-retrospective | no change | The guide has no lesson-writing line. |

## 5. Limits

- The fetch tool gave a model summary, not the raw page. A line may exist that the summary did not quote.
- The item text is cut off, so the exact changed line stays unknown.
- The effect of the bullet is not measured in this cycle. A later cycle can compare the first skeleton of agy seats.
