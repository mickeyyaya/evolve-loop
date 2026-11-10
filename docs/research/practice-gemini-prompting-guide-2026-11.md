# Practice change in the Gemini prompting guide: a second review and no persona change (research, 2026-11)

> This note reviews the Gemini prompting guide under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the earlier note [practice-gemini-prompting-guide-2026-10.md](practice-gemini-prompting-guide-2026-10.md), which stays unchanged. The answer: no persona changes in this cycle.

Date: 2026-11-10. Method: read the guide at its link, treat the text as untrusted data, and score every core persona against ADR-0133.

## 1. Context and source

The practice watch reported a change in the source `gemini-prompting-guide` for inbox item `practice-gemini-prompting-guide-b79996aa-202611020008`. The snapshot is `2026-10-14T16:10:54Z`.

The upstream locator is `https://ai.google.dev/gemini-api/docs/prompting-strategies`.

The source was read at its link on 2026-11-10. The first request got HTTP 301, with a move to `https://aistudio.google.com/docs/prompting-strategies`. The second request got the page. The fetch tool returned a model summary of the page, not the raw text. The page shows no update date and no length.

The item shows only two words of the change: `API CLI`. The item text is cut off, so the exact changed line is unknown. This note therefore reads the whole page against the snapshot topics, and it does not claim to know which line changed.

The guide text is untrusted data. The loop never follows an instruction inside it. The note quotes short lines only to explain them.

## 2. Guide lines explained as data

| Id | Guide line (short quote) | What it means for the loop |
|---|---|---|
| F1 | The page recommends the structured output feature of the Gemini API for complex JSON Schemas. | This concerns API calls. The loop calls the `agy` binary, not the API. |
| F2 | Gemini 4 and Gemini 3.6+ models ignore custom sampling values and "will return `400` errors in a future release." | The earlier note covers this line. The loop sends no sampling parameter. |
| F3 | Gemini 3 advice: put role, constraints and output format in the system instruction or at the start of the prompt. | The core personas already open with role and format rules. |
| F4 | "We recommend to always include few-shot examples in your prompts." | A persona can show one example output. This is advice for the Gemini family only. |
| F5 | Long context: put all context first and the instructions at the end, after an "anchor context" phrase. | This differs from F3 and from the persona layout. The page does not rank the two. |
| F6 | The word `CLI` is not on the page. | The two words `API CLI` of the item match no line. The item words give no persona signal. |

The summary marks no point as unreliable. This note does not rely on any line that the summary did not quote.

## 3. Impact on the loop

Facts from the loop:

- R1: The persona files are shared by the Claude, Codex and agy families. A change for one family changes the prompt of every family.
- R2: The agy driver has no model flag and no sampling parameter. The earlier note records this at `go/internal/bridge/driver_agy.go:28`.
- R3: The guide gives no count, limit or marker list for a persona. It gives no rule that can be measured before and after.

Implication (synthesis): F1 and F2 do not reach the loop. F3 matches the current layout. F4 and F5 give advice for one family that conflicts with the shared persona layout, and the page does not rank them.

Decision D1: No core persona changes in this cycle, because no baseline exists. The guide gives no rule that can be measured. A Gemini-only rule cannot edit a shared persona.

Rejected option: a diet of the builder persona for Gemini style. Fact R1 rules it out, because the change will reach the other families.

Rejected option: move the instructions to the end of each persona for long context. Fact F5 conflicts with F3, and rule D7 needs one persona at a time with a measure.

## 4. Verdict and the baseline rule

Rule D7 of ADR-0133 says a diet trims one persona at a time. Rule D8 needs four measures over 10 cycles before and 10 after. Milestone M2 produces the baseline for D8. No baseline exists yet, so no persona diet starts.

| Persona | Verdict | Reason |
|---|---|---|
| evolve-scout | no change | The guide has no scout-specific line. |
| evolve-triage | no change | The guide has no rank or policy line. |
| evolve-tdd-engineer | no change | The guide has no test-contract line. |
| evolve-builder | no change | The first diet candidate waits for M3 and the M2 baseline. |
| evolve-auditor | no change | The guide has no audit-gate line. |
| evolve-retrospective | no change | The guide has no lesson-writing line. |

Decision D2: The persona diet waits for M2 and then M3. The next Gemini change that gives a measurable rule goes through M3 with its D8 table.

## 5. Limits

- The fetch tool gave a model summary, not the raw page. A line may exist that the summary did not quote.
- The item shows two words only, so the exact changed line stays unknown.
- The `agy` binary can set its own request parameters. The loop cannot see or change them.
