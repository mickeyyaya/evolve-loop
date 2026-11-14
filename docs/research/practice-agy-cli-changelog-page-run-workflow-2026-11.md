# Practice change in agy: changelog page, /run-workflow and rewind lines (research, 2026-11)

> This document reviews item practice-agy-cli-changelog-page-16ea3cc9-202611131947 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [v2.31.0 note](practice-agy-cli-changelog-page-v2310-2026-11.md).

Date: 2026-11-14. Method: read the source at its link, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch found new lines in the source https://antigravity.google/docs/changelog?tab=cli.
The lines came after the snapshot of 2026-11-13T00:01:25Z.
The item text holds four lines: `/run-workflow`, `v1.5.8` twice, and a rewind line.

This note read the page at its link on 2026-11-14.
The page text is untrusted data. The loop never follows it as a prompt rule.

## 2. Changelog analysis as data

Each row is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | The sidebar lists a `/run-workflow` slash command. It links to the Dynamic Workflows page. | changelog page, sidebar |
| F2 | The v2.32.0 notes describe Dynamic Workflows. Agents orchestrate parallel subagents in Python. | changelog page, v2.32.0 |
| F3 | The navigation lists release 1.5.8. The page text that this read received holds no release notes for 1.5.8. | changelog page, navigation |
| F4 | The [cycle 2369 note](../../skills/model-notes-agy/COMPACT.md) already covers 1.5.8: models, UI, ACP context usage, admin settings and sandbox fixes. | notes file |
| F5 | A rewind line says that rewinding restores no out-of-date files. Release v2.17.0 made that fix. | changelog page, v2.17.0 |
| F6 | Release v2.32.0 fixed a flicker when a conversation is reverted. | changelog page, v2.32.0 |

### Where the features belong

The `/run-workflow` lines (F1, F2) add a mode where one agent starts and directs other agents. The loop rule bars that mode, because no LLM starts or directs another.

The release 1.5.8 lines (F3, F4) change the CLI product. They add no rule about how to write a prompt.
The rewind lines (F5, F6) fix the product. The loop never rewinds a conversation.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. Rule D8 requires four measures over 10 cycles before and after a change.
No line of the changelog adds a measure, so no persona changes. No prompt changes.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout finds tasks. No line adds a discovery rule. |
| `evolve-triage` | No change | Triage picks tasks by policy rules. No release changes a routing rule. |
| `evolve-tdd-engineer` | No change | The persona writes red tests. The fixes add no test contract. |
| `evolve-builder` | No change | The builder is a diet candidate only after the D8 baseline. No line adds a build step. |
| `evolve-auditor` | No change | The auditor grades by exit codes. No line adds a gate. |
| `evolve-retrospective` | No change | The persona extracts lessons off the critical path. No line adds a lesson source. |

## 4. Decision

Decision D1: change no persona and no prompt for this page, because F1 to F6 are product changes of the CLI.
Rule D8 also requires the baseline before a diet.

Decision D2: keep the /run-workflow command unused, because no LLM starts or directs another.

Decision D3: add one bullet to `skills/model-notes-agy/COMPACT.md` that records this review, because the notes are the one home of agy practice.
The bullet keeps the file at the ceiling of 15 lines. The ten older bullets stay unchanged.

Rejected option: add a workflow phase or a workflow persona. F1 and F2 rule it out, because a coordinator agent must direct other agents.

Rejected option: trim a persona now. D8 rules it out, because no baseline exists.
