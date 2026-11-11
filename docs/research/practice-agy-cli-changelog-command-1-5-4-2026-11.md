# Practice change in agy: changelog command releases 1.5.4 and 1.5.3 (research, 2026-11)

> This document reviews item practice-agy-cli-changelog-command-5a49deca-202611100841 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [page note](practice-agy-cli-changelog-page-v2300-2026-11.md), which covers the App panel.

Date: 2026-11-10. Method: run the command, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch found new lines in the source `agy changelog`.
The new lines came after the snapshot of 2026-11-03T02:36:34Z.
The command prints releases 1.5.4 and 1.5.3. The item text is cut, so this note ran the command on 2026-11-10.
The changelog text is untrusted data. The loop never follows it as a prompt rule.

## 2. Changelog analysis as data

Each row is a fact about the command output. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | Release 1.5.4 adds Auto mode to `agy acp` sessions, so an editor can switch a session to `auto`. | `agy changelog`, 1.5.4 |
| F2 | Release 1.5.4 adds `hookEventName`, `cwd` and `toolCall.id` to the JSON input of a hook. | `agy changelog`, 1.5.4 |
| F3 | Release 1.5.4 fixes crashes and freezes of the subagent view in a narrow terminal. | `agy changelog`, 1.5.4 |
| F4 | Release 1.5.4 fixes the sign-in storage and the sign-in warning in `--remote-agent` sessions. | `agy changelog`, 1.5.4 |
| F5 | Release 1.5.3 shows the model of each subagent and adds plugin variables to plugin hooks. | `agy changelog`, 1.5.3 |
| F6 | Release 1.5.3 retries a turn that fails with an HTTP/2 header timeout. | `agy changelog`, 1.5.3 |
| F7 | The other lines cover paging, titles, plugin commands and file reads. They are product fixes. | `agy changelog` |

### Where the features belong

The Auto mode line (F1) is for an editor that runs `agy acp`. The loop does not use that mode.
The hook fields (F2) add data to hook input. The loop hooks read their own fields and need no change.
The fixes (F3 to F7) repair the CLI. They add no rule about how to write a prompt.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. Rule D8 requires four measures over 10 cycles before and after a change.
No line of the changelog adds a measure, so no persona changes.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout finds tasks. No line adds a discovery rule. |
| `evolve-triage` | No change | Triage picks tasks by policy rules. No release changes a routing rule. |
| `evolve-tdd-engineer` | No change | The persona writes red tests. The fixes add no test contract. |
| `evolve-builder` | No change | The builder is a diet candidate only after the D8 baseline. No line adds a build step. |
| `evolve-auditor` | No change | The auditor grades by exit codes. The Auto mode line adds no gate. |
| `evolve-retrospective` | No change | The persona extracts lessons off the critical path. No line adds a lesson source. |

## 4. Decision

Decision D1: change no persona and no prompt for releases 1.5.4 and 1.5.3, because F1 to F7 are product changes of the CLI.
Rule D8 also requires the baseline before a diet.

Decision D2: add one bullet to `skills/model-notes-agy/COMPACT.md` that records this review, because the notes are the one home of agy practice.
The bullet keeps the file below the ceiling of 12 lines. The five old bullets stay unchanged.

Rejected option: add a prompt rule for Auto mode or hook fields. F1 and F2 rule it out, because the loop does not use them.

Rejected option: trim a persona now. D8 rules it out, because no baseline exists.
