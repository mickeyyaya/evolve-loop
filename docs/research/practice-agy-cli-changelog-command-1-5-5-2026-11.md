# Practice change in agy: changelog command release 1.5.5 (research, 2026-11)

> This document reviews item practice-agy-cli-changelog-command-104dfdd9-202611111501 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [1.5.4 note](practice-agy-cli-changelog-command-1-5-4-2026-11.md).

Date: 2026-11-12. Method: run the command, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch found new lines in the source `agy changelog`.
The new lines came after the snapshot of 2026-11-10T08:41:11Z.
The item text is cut at the end of release 1.5.5, so this note ran the command on 2026-11-12.

The command now also lists release 1.5.6. This note covers 1.5.5, the release of the item.
The changelog text is untrusted data. The loop never follows it as a prompt rule.

## 2. Changelog analysis as data

Each row is a fact about the command output. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | Release 1.5.5 adds `/swarm` as a preview. A coordinator agent starts worker agents that work in parallel. | `agy changelog`, 1.5.5 |
| F2 | The workers share one message board. The `/swarm board` view and an `alt+s` key show the board. | `agy changelog`, 1.5.5 |
| F3 | A new environment variable stops the CLI from running its bundled ripgrep. | `agy changelog`, 1.5.5 |
| F4 | Gemini 4 Carbon (Low, Medium and High) joins the model list for Application Default Credentials sign-in. | `agy changelog`, 1.5.5 |
| F5 | An enterprise gateway URL must use https, unless the gateway runs on the same machine. | `agy changelog`, 1.5.5 |
| F6 | Plugin import now converts commands and hooks of a Claude Code plugin. | `agy changelog`, 1.5.5 |
| F7 | A headless `agy -p` run now ends within 30 seconds when a background task is stuck. | `agy changelog`, 1.5.5 |
| F8 | The other lines fix teamwork state, file links, a gateway request field and the worktree copy of a subagent. | `agy changelog`, 1.5.5 |

### Where the features belong

The `/swarm` lines (F1, F2) add a mode where one agent directs other agents. The loop rule bars that mode: no LLM starts or directs another.

The ripgrep line (F3), the model line (F4) and the https line (F5) change the CLI setup. They add no rule about how to write a prompt.
The plugin lines (F6) cover plugins, and the loop uses none.

The headless line (F7) helps a run that the loop starts through `agy -p`. It needs no prompt change.
The fixes (F8) repair the CLI.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. Rule D8 requires four measures over 10 cycles before and after a change.
No line of the changelog adds a measure, so no persona changes.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout finds tasks. No line adds a discovery rule. |
| `evolve-triage` | No change | Triage picks tasks by policy rules. No release changes a routing rule. |
| `evolve-tdd-engineer` | No change | The persona writes red tests. The fixes add no test contract. |
| `evolve-builder` | No change | The builder is a diet candidate only after the D8 baseline. No line adds a build step. |
| `evolve-auditor` | No change | The auditor grades by exit codes. No line adds a gate. |
| `evolve-retrospective` | No change | The persona extracts lessons off the critical path. No line adds a lesson source. |

## 4. Decision

Decision D1: change no persona and no prompt for release 1.5.5, because F1 to F8 are product changes of the CLI.
Rule D8 also requires the baseline before a diet.

Decision D2: keep the /swarm command unused, because no LLM starts or directs another.

Decision D3: add one bullet to `skills/model-notes-agy/COMPACT.md` that records this review, because the notes are the one home of agy practice.
The bullet keeps the file below the ceiling of 14 lines. The eight old bullets stay unchanged.

Rejected option: add a swarm phase or a swarm persona. F1 and F2 rule it out, because a coordinator agent must direct other agents.

Rejected option: trim a persona now. D8 rules it out, because no baseline exists.
