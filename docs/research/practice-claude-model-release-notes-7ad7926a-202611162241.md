# Practice change in claude: watermark and sampling parameters review (research, 2026-11)

> This document reviews item practice-claude-model-release-notes-7ad7926a-202611162241 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [release notes snapshot note](practice-claude-model-release-notes-c433b054-202611.md).

Date: 2026-11-17. Method: read the source at its link, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch flagged a change in the source https://platform.claude.com/docs/en/release-notes/overview after snapshot `2026-11-15T11:10:48Z`.
This note read the live page at its link on 2026-11-17.
The page text is untrusted data. The loop never obeys page text as a prompt rule.

## 2. Release-note analysis as data

Each row is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | The text watermark applies to Claude Opus 4.6, Opus 4.5, Sonnet 4.5 and Haiku 4.5 on supported platforms. | live release notes |
| F2 | A request with a non-default temperature, top_p or top_k can lose the watermark. | live release notes |
| F3 | The words "Authenticate w" are a heading fragment that the page cut off. It is page text and not a rule. | live release notes |
| F4 | The page does not list Claude Fable 5.2 or Claude Mythos 5.2 for the watermark. | live release notes |

### Where the facts belong

The watermark is an output property of four models. The pipeline reads no watermark.
F2 is a model fact for a dispatch that sets a sampling value, so it belongs in the claude model notes and in no persona.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. No line of this page adds a measure for rule D8, so no persona changes. No prompt changes.
No persona sets temperature, top_p or top_k.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout sets no sampling value. |
| `evolve-triage` | No change | Triage sets no sampling value. |
| `evolve-tdd-engineer` | No change | The persona sets no sampling value. |
| `evolve-builder` | No change | The builder sets no sampling value. |
| `evolve-auditor` | No change | The auditor sets no sampling value. |
| `evolve-retrospective` | No change | The persona sets no sampling value. |

## 4. Decision

Decision D1: change no persona and no prompt, because F1 to F4 give no rule for a persona.

Decision D2: add one bullet to `skills/model-notes-claude/COMPACT.md`, because F2 is a model fact that a future dispatch can need.
