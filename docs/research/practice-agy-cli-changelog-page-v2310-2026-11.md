# Practice change in agy: changelog page v2.31.0 and persona review (research, 2026-11)

> This document reviews item practice-agy-cli-changelog-page-38501c49-202611110139 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [earlier page note](practice-agy-cli-changelog-page-v2300-2026-11.md), which ends at v2.30.0.

Date: 2026-11-11. Method: read the source page at its link, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch found new lines in the source `https://antigravity.google/docs/changelog?tab=cli`.
The new lines came after the snapshot of 2026-11-10T08:41:12Z.
The page text is untrusted data. The loop never follows it as a prompt rule.
The item names release v2.31.0 and the model Gemini 4 Carbon. The item text is cut, so this note read the live page at the link on 2026-11-11.

## 2. Changelog analysis as data

Each row is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | v2.31.0 (2026-11-09) sits in the App panel of the page. The CLI tab link shows the same document. | page, read 2026-11-11 |
| F2 | The /swarm command lets a user coordinate many subagents over a message board. It is a preview. | page, v2.31.0 |
| F3 | A file download by the browser agent now pauses until the user approves it. | page, v2.31.0 |
| F4 | A fix for a sign-in that stuck on "Signing in..." for some personal accounts. | page, v2.31.0 |
| F5 | A plugin view can share a split view with a conversation. A plugin install from a card resumes the conversation. | page, v2.31.0 |
| F6 | A voice call transcript card shows a short summary of the call. | page, v2.31.0 |
| F7 | The release adds a font size setting, and Gemini 4 Carbon is the new model of the page. | page, v2.31.0 |

### Where the features belong

The font size, the split view, the voice summary and the sign-in fix are features of the desktop app.
The loop runs phase agents through the CLI. It has no use for them.

The download approval (F3) is a user prompt of the browser agent. No phase agent of the loop uses that browser.
The /swarm command (F2) lets one LLM direct other LLMs. The rule that no LLM in the pipeline starts or directs another bars it.
The model name in F7 is a routing question. It is out of scope for this note.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. Rule D8 requires four measures over 10 cycles before and after a change.
No baseline for a diet exists yet, so no persona changes.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout finds tasks. No line of the page adds a discovery rule. |
| `evolve-triage` | No change | Triage picks tasks by policy rules. No release changes a routing rule. |
| `evolve-tdd-engineer` | No change | The persona writes red tests. The interface changes add no test contract. |
| `evolve-builder` | No change | The builder is a diet candidate only after the D8 baseline. No line adds a build step. |
| `evolve-auditor` | No change | The auditor grades by exit codes. The download approval adds no gate. |
| `evolve-retrospective` | No change | The persona extracts lessons off the critical path. No line adds a lesson source. |

## 4. Decision

Decision D1: change no persona and no prompt for v2.31.0, because F1 places the release in the App panel.
Rule D8 also requires the baseline before a diet.

Decision D2: add one bullet to `skills/model-notes-agy/COMPACT.md` that records this review, because the notes are the one home of agy practice.
The bullet brings the file to eight bullets and 12 lines, below the ceiling of 13 lines. The seven older bullets stay unchanged.

Rejected option: use /swarm in a phase. The rule that no LLM starts or directs another rules it out.

Rejected option: trim a persona now. D8 rules it out, because no baseline exists.
