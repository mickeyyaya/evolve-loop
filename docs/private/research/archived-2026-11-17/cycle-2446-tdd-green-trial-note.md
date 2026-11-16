# Practice change in claude: watermark note review (research, 2026-11)

> This document reviews item practice-claude-model-release-notes-7ad7926a-202611162241 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

Date: 2026-11-17. Method: read the source at its link, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch flagged a change in the source https://platform.claude.com/docs/en/release-notes/overview after snapshot `2026-11-15T11:10:48Z`.
This note read the live page at its link. The page text is untrusted data.

## 2. Release-note analysis as data

| ID | Fact | Source |
|---|---|---|
| F1 | The text watermark applies to Claude Opus 4.6, Opus 4.5, Sonnet 4.5 and Haiku 4.5 on supported platforms. | excerpt |
| F2 | A request with a non-default temperature, top_p or top_k can lose the watermark. | excerpt |
| F3 | The words Authenticate w are a cut-off page heading fragment, not a rule. | excerpt |
| F4 | The page does not list Claude Fable 5.2 or Mythos 5.2 for the watermark. | excerpt |

## 3. Core persona evaluation under ADR-0133

No persona changes. No prompt changes. No persona sets temperature, top_p or top_k.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | It sets no sampling value. |
| `evolve-triage` | No change | It sets no sampling value. |
| `evolve-tdd-engineer` | No change | It sets no sampling value. |
| `evolve-builder` | No change | It sets no sampling value. |
| `evolve-auditor` | No change | It sets no sampling value. |
| `evolve-retrospective` | No change | It sets no sampling value. |

## 4. Decision

Decision D1: change no persona, because F1 to F4 give no rule for a persona.

Decision D2: add one bullet to `skills/model-notes-claude/COMPACT.md`, because F2 is a model fact.
