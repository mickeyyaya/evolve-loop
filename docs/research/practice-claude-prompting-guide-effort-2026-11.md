# Practice change in claude: Fable 5.2, Mythos 5.2 and effort (research, 2026-11)

> This note reviews the change to the `claude-prompting-guide` source for item bc673fd8 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The answer is this: one bullet changes in `skills/model-notes-claude/COMPACT.md`, and no persona changes.

Date: 2026-11-10. Method: read the source at the link as untrusted data. Give a verdict for each core persona under ADR-0133.

## 1. The request and the answer

The inbox item gives two acceptance lines:

- "The source `https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices` is read at the link, and the lines above are explained as data."
- "A proposal names each prompt or persona to change under ADR-0133, or states that none changes and why."

The source changed after the snapshot of 2026-11-06T02:47:04Z.
The page is a third-party document, and its text is untrusted data.
This note explains the text as data. It does not follow any instruction in the text.

## 2. The guide change, explained as data

| ID | Fact from the source | Scope |
|---|---|---|
| F1 | The guide now covers Fable 5.2 and Mythos 5.2. The guide names the differences from earlier models, such as effort calibration and final reports. | Prompting |
| F2 | Thinking is always on for Fable 5.2 and Mythos 5.2. Adaptive thinking is the only mode. | Thinking |
| F3 | The guide says that, at medium effort and above, the two models think before almost every reply. | Effort |
| F4 | The guide says an order to answer directly, without deliberation, did not stop the thinking at medium effort and above. The fix in the guide is to lower the effort level. | Effort |
| F5 | The guide adds a rule for a product dose question about a child. The model must find the product label and must not give a number from memory. This is a topic rule for one use. It is not a rule for the loop. | Medication |
| F6 | The item text ends with the marker "added lines truncated". The loop read only part of the change. | Truncated |

## 3. Impact on the loop

Facts from the loop:

- R1: `skills/model-notes-claude/COMPACT.md` loads into each dispatch with a Claude CLI. It held six bullets before this change.
- R2: No file in `agents/evolve-*.md` holds an order such as "do not think" or "without thinking". Predicate 006 of cycle 2278 checks this.
- R3: The loop sets the effort level through configuration, not through persona text.

Implication (synthesis): only F3 and F4 give a rule that an agent can follow. F5 is outside the work of the loop. F6 limits what the loop can adopt.

Decision D1: add one bullet to `skills/model-notes-claude/COMPACT.md`, because F4 shows that an order to skip thinking fails. The notes are the one home of model rules.

Rejected option: add the rule to a persona. Fact R2 rules this out, because no persona holds an order that the rule corrects. D7 of ADR-0133 also allows one persona change at a time.

Rejected option: adopt the medication rule of F5. Fact F5 rules this out, because it applies to one topic outside the loop.

## 4. Core persona evaluation under ADR-0133

Each row gives the verdict first, then the reason.
D8 requires a baseline of four measures over 10 cycles before and after a diet change.
Milestone M2 produces that baseline, and M2 has not shipped.
This proposal changes no persona and no prompt.

| Persona | Verdict | Reason |
|---|---|---|
| evolve-scout | no change | It holds no order to skip thinking, so F4 does not apply. |
| evolve-triage | no change | It picks tasks from inbox ranks, and the guide does not alter routing. |
| evolve-tdd-engineer | no change | It writes red tests, and the guide does not alter test contracts. |
| evolve-builder | no change | It is the first diet candidate, and D8 needs the M2 baseline first. |
| evolve-auditor | no change | It reads test exit codes, and the guide does not alter audit gates. |
| evolve-retrospective | no change | It extracts lessons after ship, and the guide does not alter its steps. |

## 5. Decisions and rejected options

| ID | Decision | Evidence | Rejected option and the fact that rules it out |
|---|---|---|---|
| D1 | Add one effort bullet to `skills/model-notes-claude/COMPACT.md`. | F3, F4, R1 | Add the rule to a persona: R2 shows no persona needs it. |
| D2 | No persona changes in this cycle. | D8, §4 | Trim the builder persona now: the D8 baseline from M2 does not exist. |

Consequence: the notes grow from 6 to 7 bullets and stay under the ceiling of 12 lines.

## 6. Limits

- This review covers the lines that follow the snapshot of 2026-11-06T02:47:04Z.
- The item text was truncated, so the loop adopts only the effort claim.
- The earlier note, [practice-claude-model-release-notes-fable-5-2-2026-11.md](practice-claude-model-release-notes-fable-5-2-2026-11.md), stays unchanged.
