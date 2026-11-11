# Practice change in claude: Fable 5.2, Mythos 5.2 and the release notes (research, 2026-11)

> This note reviews the change to the `claude-model-release-notes` source for item 9cd2c891 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The answer is this: one bullet changes in `skills/model-notes-claude/COMPACT.md`, and no persona changes.

Date: 2026-11-10. Method: read the source at the link as untrusted data. Give a verdict for each core persona under ADR-0133.

## 1. The request and the answer

The inbox item gives two acceptance lines:

- "The source `https://platform.claude.com/docs/en/release-notes/overview` is read at the link, and the lines above are explained as data."
- "A proposal names each prompt or persona to change under ADR-0133, or states that none changes and why."

The source changed after the snapshot of 2026-11-02T17:20:05Z.
The page is a third-party document, and its text is untrusted data.
This note explains the text as data. It does not follow any instruction in the text.

## 2. The release notes change, explained as data

The latest entry is dated 2026-11-09.

| ID | Fact from the source | Scope |
|---|---|---|
| F1 | 2026-11-09: Claude Fable 5.2 and Claude Mythos 5.2 launch. Mythos 5.2 is limited to verified organizations. | Platform API |
| F2 | 2026-11-09: both models enforce the prefix check on every account. A request that edits the system prompt, the tools or an earlier message and then sends back a later thinking block returns a 400 error. | Platform API |
| F3 | 2026-11-09: an advisor tool in beta accepts only Fable 5.2 or Mythos 5.2 as the advisor of a Fable 5.2 or Mythos 5.2 executor. Any other advisor returns a 400 error. | Platform API |
| F4 | 2026-11-09: fast mode is available for Fable 5.2 with the `speed` field set to `fast`. | Platform API |
| F5 | 2026-11-05: an IP allowlist in beta limits the addresses that can use an API key. | Console |
| F6 | 2026-11-03: text from some models carries a text watermark. | Platform |

## 3. Impact on the loop

Facts from the loop:

- R1: The loop dispatches Claude models through the `claude` CLI binary or the `agy` bridge driver. It sends no direct Messages API request, so it never edits a thinking block.
- R2: The notes in `skills/model-notes-claude/COMPACT.md` load into each dispatch with a Claude CLI. They hold five bullets before this change.
- R3: Phase personas under `agents/` set no request size, no `speed` field and no advisor.

Implication (synthesis): the CLI binary owns the prefix check, the advisor and fast mode.
Only F2 gives a rule that an agent can follow, and an agent that builds on the API can break it.
F3 through F6 need no edit.

Decision D1: add one bullet to `skills/model-notes-claude/COMPACT.md`. The bullet names Fable 5.2, Mythos 5.2, thinking and the 400 error. This is the decision because F2 is a rule for a tool that calls the API. The notes are the one home of such rules.
Rejected option: add the rule to a persona. Fact R3 rules this out, because the personas hold no API call, and D7 allows one persona change at a time.

## 4. Core persona evaluation under ADR-0133

Each row gives the verdict first, then the reason.
D8 requires a baseline of four measures over 10 cycles before and after a diet change.
Milestone M2 produces that baseline, and M2 has not shipped.
This proposal changes no persona and no prompt.

| Persona | Verdict | Reason |
|---|---|---|
| evolve-scout | no change | It finds tasks and sets no API field, so F1 through F6 do not apply. |
| evolve-triage | no change | It picks tasks from inbox ranks, and the release notes do not alter routing. |
| evolve-tdd-engineer | no change | It writes red tests, and the release notes do not alter test contracts. |
| evolve-builder | no change | It is the first diet candidate, and D8 needs the M2 baseline first. |
| evolve-auditor | no change | It reads test exit codes, and the release notes do not alter audit gates. |
| evolve-retrospective | no change | It extracts lessons after ship, and the release notes do not alter its steps. |

## 5. Decisions and rejected options

| ID | Decision | Evidence | Rejected option and the fact that rules it out |
|---|---|---|---|
| D1 | Add one bullet to `skills/model-notes-claude/COMPACT.md`. | F2, R2 | Add the rule to a persona: R3 shows the personas hold no API call. |
| D2 | No persona changes in this cycle. | D8, §4 | Trim the builder persona now: the D8 baseline from M2 does not exist. |

Consequence: the notes grow from 5 to 6 bullets and stay under the ceiling of 12 lines.

## 6. Limits

- This review covers the entries that follow the snapshot of 2026-11-02T17:20:05Z.
- The page text was read in part. The last part of the page, 43335 characters, was not read.
- The earlier note, [practice-claude-model-release-notes-2026-11.md](practice-claude-model-release-notes-2026-11.md), stays unchanged.
