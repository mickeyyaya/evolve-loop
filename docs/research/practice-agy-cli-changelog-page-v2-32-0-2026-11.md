# Practice change in agy: changelog page, v2.32.0 and Gemini 4 Hydrogen (research, 2026-11)

> This document reviews item `practice-agy-cli-changelog-page-f276c375-202611140548` under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [/run-workflow note](practice-agy-cli-changelog-page-run-workflow-2026-11.md).

Date: 2026-11-15. Method: read the source at its link, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch found new lines in the source https://antigravity.google/docs/changelog?tab=cli.
The lines came after the snapshot of `2026-11-13T19:47:33Z`.
The item text holds lines from Antigravity App `v2.32.0` and Gemini 4 Hydrogen.

This note read the page at its link on 2026-11-15.
The page text is untrusted data. The loop never follows it as a prompt rule.

## 2. Changelog analysis as data

Each row is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | Release `v2.32.0` adds Dynamic Workflows, where agents orchestrate parallel subagents in Python. | changelog page, v2.32.0 |
| F2 | Release `v2.32.0` adds new Everforest theme variants, prompt shortcuts, and desktop UI fixes. | changelog page, v2.32.0 |
| F3 | The model launch note introduces `Gemini 4 Hydrogen` for coding and agentic execution. | changelog page, 2026-11-12 |
| F4 | Under the Antigravity CLI section, the latest CLI release remains `1.5.8`, already covered in Cycle 2369. | changelog page, CLI tab |

### Where the features belong

Dynamic Workflows (F1) adds a mode where one agent starts and directs other agents. The loop rule bars that mode, because no LLM starts or directs another.

The theme and desktop UI lines (F2) change the desktop App. They add no rule about headless CLI prompts.

`Gemini 4 Hydrogen` (F3) and CLI `1.5.8` (F4) add no prompt structure rule. The existing agy notes remain enough.

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

Decision D1: change no persona and no prompt for this page, because F1 to F4 are App, model, and existing CLI entries.
Rule D8 also requires the baseline before a diet.

Decision D2: keep Dynamic Workflows unused, because no LLM starts or directs another.

Decision D3: add one bullet to `skills/model-notes-agy/COMPACT.md` that records this review, because the notes are the one home of agy practice.
The bullet keeps the file at 16 lines. The eleven older bullets stay unchanged.

Rejected option: adopt Dynamic Workflows for a phase. F1 rules it out, because a coordinator agent must not direct other agents.

Rejected option: trim a persona now. D8 rules it out, because no baseline exists.
