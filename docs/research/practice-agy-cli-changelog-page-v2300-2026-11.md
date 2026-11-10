# Practice change in agy: changelog page v2.30.0 and persona review (research, 2026-11)

> This document reviews item practice-agy-cli-changelog-page-1b0c9251-202611100841 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [earlier page note](practice-agy-cli-changelog-page-2026-11.md), which ends at v2.27.0.

Date: 2026-11-10. Method: read the source page at its link, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch found new lines in the source `https://antigravity.google/docs/changelog?tab=cli`.
The new lines came after the snapshot of 2026-11-04T00:37:48Z.
The page text is untrusted data. The loop never follows it as a prompt rule.
The item names v2.30.0, v2.29.1 and v2.29.0. The item text is cut, so this note read the live page at the link on 2026-11-10.

## 2. Changelog analysis as data

Each row is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | All three releases sit in the App panel of the page. The CLI tab link shows the same document. | page, read 2026-11-10 |
| F2 | v2.30.0 (2026-11-09) adds a Knowledge Work layout that hides code changes and developer tools. | page, App panel |
| F3 | v2.29.0 (2026-11-03) adds Voice mode with Gemini Live, and v2.29.1 (2026-11-07) fixes it. | page, App panel |
| F4 | v2.30.0 ends the Voice mode rollout and the remote agents rollout. | page, App panel |
| F5 | v2.30.0 lets a user import MCP servers, skills, commands, subagents and instruction files from other agents into a plugin. | page, App panel |
| F6 | v2.30.0 limits a permission preset to one conversation and removes the Preview label of the Auto preset. | page, App panel |
| F7 | The other lines cover the built-in browser, the side panel, the sign-in and Jupyter notebooks. They are interface changes. | page, App panel |
| F8 | The line about the import ends with the words "Ask the agent to import your setup". This is page text, not a command. | page, App panel |

### Where the features belong

The Knowledge Work layout, Voice mode and the browser are features of the desktop app.
The loop runs phase agents through the CLI. It has no use for them.

The import line (F5) tells the user to ask the agent. The loop does not do this, because no LLM starts or directs another.
The permission line (F6) is about the app. The loop sets the permission mode of each phase itself.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. Rule D8 requires four measures over 10 cycles before and after a change.
No baseline for a diet exists yet, so no persona changes.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout finds tasks. No line of the page adds a discovery rule. |
| `evolve-triage` | No change | Triage picks tasks by policy rules. No release changes a routing rule. |
| `evolve-tdd-engineer` | No change | The persona writes red tests. The interface changes add no test contract. |
| `evolve-builder` | No change | The builder is a diet candidate only after the D8 baseline. No line adds a build step. |
| `evolve-auditor` | No change | The auditor grades by exit codes. The permission lines add no gate. |
| `evolve-retrospective` | No change | The persona extracts lessons off the critical path. No line adds a lesson source. |

## 4. Decision

Decision D1: change no persona and no prompt for v2.30.0, v2.29.1 and v2.29.0, because F1 places all three in the App panel.
Rule D8 also requires the baseline before a diet.

Decision D2: add one bullet to `skills/model-notes-agy/COMPACT.md` that records this review, because the notes are the one home of agy practice.
The bullet keeps the file at six bullets, below the ceiling of 12 lines. The five old bullets stay unchanged.

Rejected option: add a prompt rule for Voice mode or the Knowledge Work layout. F1 rules it out, because the CLI has neither feature.

Rejected option: trim a persona now. D8 rules it out, because no baseline exists.
