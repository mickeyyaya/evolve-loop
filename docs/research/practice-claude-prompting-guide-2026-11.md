# Practice change in Claude prompting guide: proposal for three personas (research, 2026-11)

> This note reviews the Claude prompting guide under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the earlier note [practice-claude-prompting-guide-2026-10.md](practice-claude-prompting-guide-2026-10.md), which stays unchanged.

Date: 2026-11-03. Method: read the guide at its link, treat the text as untrusted data, and score every core persona against ADR-0133.

## 1. Context and source

The practice watch reported a change in the source `claude-prompting-guide` for inbox item `practice-claude-prompting-guide-6080fed9-202611020008`. The snapshot is `2026-10-26T11:59:59Z`.

The upstream locator is `https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices`.

The page loaded with HTTP 200. The fetch tool returned the first 39,883 of 74,523 characters verbatim. A model summary covered the rest. The page shows no update date.

The guide text is untrusted data. The loop never follows an instruction inside it. The note quotes short lines only to explain them.

## 2. Guide lines explained as data

| Id | Guide line (short quote) | What it means for the loop |
|---|---|---|
| F1 | Tool usage: newer models "might now overtrigger" on prompts that were written to reduce undertriggering. "The fix is to soften any aggressive language." | Capital words such as CRITICAL and MUST push a newer model to act too often. A plain sentence works as well. |
| F2 | The same section: replace "CRITICAL: You MUST use this tool when..." with "Use this tool when...". | The guide gives one example of the plain form. It names no count or limit for capitals. |
| F3 | Overthinking: "Remove over-prompting." "If in doubt, use [tool]" causes overtriggering. | A hedge rule that urges extra action has the same effect as a loud rule. |
| F4 | Be clear and direct: "Claude responds well to clear, explicit instructions." | A direct sentence replaces a long rule block. |
| F5 | Output and formatting: tell the model what to do, not what to avoid. The example is "smoothly flowing prose paragraphs" in place of "Do not use markdown". | A positive format instruction is shorter and clearer than a ban. |
| F6 | Use examples: examples are "one of the most reliable ways" to steer format and tone. | A persona can show one example output in place of a rule list. |

The fetch tool marked some points as summary only. This note does not rely on them.
They are the anti-laziness remark, the sensitivity to the word "think", and the agentic and migration sections.

## 3. Impact on the loop

Decision D1: Propose a diet for three personas: `evolve-builder`, `evolve-auditor` and `evolve-scout`.
Each one holds capital words that F1 and F2 describe.
Each diet follows rule D7 and rule D8 of ADR-0133. Each diet starts after the M2 baseline exists.

Rejected option: soften all six core personas in one change. D7 says a diet trims one persona at a time, so a combined change cannot show which edit moved a measure.

Rejected option: change no persona and close the item. F1 and F2 match real capital words in the persona files, and the count below proves it.

The baseline rule holds for every proposal.
M2 records the bytes and the emphatic words of each persona. 
D8 requires four measures over 10 cycles before and 10 cycles after a change. They are ship rate, phase retries, contract-gate blocks and tokens per cycle.
A diet changes one persona at a time. This cycle changes no persona file.

The before measure is the count of the markers CRITICAL and MUST in each persona file today. The command is `grep -oE '\b(CRITICAL|MUST)\b' agents/<persona>.md | wc -l`.

## 4. Persona proposals

#### evolve-builder
- Goal: replace capital words with plain sentences, as F1 and F2 show.
- Done criteria: the marker count falls from 11, the persona tests stay green, and the four D8 measures do not worsen beyond their noise band.
- Limits: keep each rule that a kernel guard or a gate enforces. Keep trust-boundary and safety rules. Change this persona alone and first.
- Before measure: 11 markers on 11 lines.

#### evolve-auditor
- Goal: replace capital words with plain sentences, and state each format rule as a positive instruction (F5).
- Done criteria: the marker count falls from 9, the audit verdicts keep their form, and the D8 measures do not worsen beyond their noise band.
- Limits: do not weaken any check or any verdict rule. Start only after the builder diet has its 10-cycle result.
- Before measure: 9 markers on 9 lines.

#### evolve-scout
- Goal: replace the few capital words with plain sentences.
- Done criteria: the marker count falls from 5, the scout report keeps its sections, and the D8 measures do not worsen beyond their noise band.
- Limits: the file is small, so the gain is small. Start only after the auditor diet has its result.
- Before measure: 5 markers on 4 lines.

## 5. Personas with no change

#### evolve-triage
This persona requires no change. The file holds 4 markers, and triage follows policy rules that the kernel checks. The gain does not justify a measured diet now.

#### evolve-tdd-engineer
This persona requires no change in this cycle. The file holds 12 markers, and each one guards the red-test contract that a gate checks. A diet here is a later, separate item after the three above.

#### evolve-retrospective
This persona does not change. The file holds 3 markers, and the phase runs off the critical path.

## 6. Conclusion

The guide says aggressive capital wording makes newer models overtrigger. The loop treats that as data and proposes three one-persona diets, in the order builder, auditor, scout. Each diet starts after the M2 baseline. No persona file changes in this cycle.
