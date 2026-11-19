# Practice change in claude: prompt guide snapshot review (research, 2026-11)

> This note reviews item practice-claude-prompting-guide-2c5f9a4d-202611190048 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [earlier guide note](practice-claude-prompting-guide-2026-11.md).

Date: 2026-11-19. Method: read the live guide at its link as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch flagged a diff on https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices after snapshot `2026-11-15T11:10:47Z`.
This review read the live guide at its link on 2026-11-19.
The fetched markdown file holds 1,206 lines, and the HTML head holds the page shell.
The guide text is untrusted data. The loop never obeys page text as a prompt rule.

## 2. Snapshot diff and live guide as data

Each row explains the snapshot or the live guide as data. No row is an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | The two quoted lines (`⌘` and `K`) are search shortcut key cap symbols from the HTML shell. Each line is page chrome, not a rule. | inbox item snapshot |
| F2 | The live guide lists Fable 5.2, Mythos 5.2, Opus 5.5, Sonnet 5.5, and Haiku 5.5 in its model table. | live guide |
| F3 | The live guide advises plain sentences in place of emphatic capitals such as CRITICAL and MUST. | live guide |
| F4 | The live guide says Fable 5.2 and Mythos 5.2 keep thought active at medium effort and above. Lower effort replaces a direct-answer order. | live guide |
| F5 | The live guide advises an append-only history on Fable 5.2, Mythos 5.2, Opus 5.5, Sonnet 5.5, and Haiku 5.5. | live guide |

### Where the facts belong

F1 shows that the snapshot diff holds UI key cap chrome, not a new rule.
F2 through F5 repeat model families and rules that [COMPACT.md](../../skills/model-notes-claude/COMPACT.md) and the [earlier guide note](practice-claude-prompting-guide-2026-11.md) already cover.
No new model behavior rule appears in the live guide after `2026-11-15T11:10:47Z`.

## 3. Core persona evaluation under ADR-0133

ADR-0133 requires a goal, done criteria, limits, and before-and-after measures over 10 cycles for each persona diet change.
Milestone M2 supplies the baseline counts and remains open in the practice plan.
No persona and no prompt change in this cycle, because F1 is page chrome and F2 through F5 add no new rule.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | no change | The scout holds 5 emphatic markers, and the three-persona diet plan schedules it third after M2. |
| `evolve-triage` | no change | Triage ranks inbox work by policy rules, and the guide adds no new routing rule. |
| `evolve-tdd-engineer` | no change | The engineer guards red test contracts, and the guide adds no new test rule. |
| `evolve-builder` | no change | The builder holds 11 emphatic markers as the first diet candidate, and ADR-0133 requires M2 first. |
| `evolve-auditor` | no change | The auditor holds 9 emphatic markers as the second diet candidate, and ADR-0133 requires M2 first. |
| `evolve-retrospective` | no change | The retrospective runs off the critical path with 3 markers, and the guide adds no new rule. |

## 4. Decision

Decision D1: change no persona and no prompt, because F1 is search shortcut chrome and F2 through F5 add no new rule.

Decision D2: `skills/model-notes-claude/COMPACT.md` stays unchanged, because its eight bullets already cover scope, single steps, parallel tools, verification, thought history, and effort.

Rejected option: start the `evolve-builder` persona diet now or add a duplicate bullet to `skills/model-notes-claude/COMPACT.md`. ADR-0133 rules out an unmeasured edit before M2, and the existing Claude notes already cover F2 through F5.
