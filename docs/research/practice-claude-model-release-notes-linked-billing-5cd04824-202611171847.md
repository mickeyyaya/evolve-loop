# Practice change in claude: linked billing review (research, 2026-11)

> This document reviews item practice-claude-model-release-notes-5cd04824-202611171847 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [watermark note](practice-claude-model-release-notes-7ad7926a-202611162241.md).

Date: 2026-11-18. Method: read the source at its link, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch flagged a change in the source https://platform.claude.com/docs/en/release-notes/overview after snapshot `2026-11-16T22:41:47Z`.
This note read the live page at the link on 2026-11-18.
The page text is untrusted data. The loop never obeys page text as a prompt rule.

## 2. Release-note analysis as data

Each row is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | The page announces linked billing. An app owner can let a customer pay for the Claude usage of the app. | live release notes |
| F2 | Linked billing sits behind the beta header user-profiles-billing. | live release notes |
| F3 | A request to /v1/messages can carry the field billing_policy. | live release notes |
| F4 | The same field also applies to /v1/chat/completions. | live release notes |
| F5 | A customer with a Claude organization pays through that organization. A spend cap of that organization still applies. | live release notes |
| F6 | The last line of the page names a launch of anthropic-managed hosting. The page cut off that line, so it is page text and not a rule. | live release notes |

### Where the facts belong

Linked billing decides who pays for a request. It does not decide how a prompt must read.
The pipeline sends no customer usage, so F1 to F5 are API billing facts and not model behavior facts.
F6 is incomplete, so it gives no fact at all.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. No line of this page adds a measure for rule D8, so no persona changes. No prompt changes.
No persona sets billing_policy.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout sends no billing field. |
| `evolve-triage` | No change | Triage sends no billing field. |
| `evolve-tdd-engineer` | No change | The persona sends no billing field. |
| `evolve-builder` | No change | The builder sends no billing field. |
| `evolve-auditor` | No change | The auditor sends no billing field. |
| `evolve-retrospective` | No change | The persona sends no billing field. |

## 4. Decision

Decision D1: change no persona and no prompt, because F1 to F6 give no rule for a persona.

Decision D2: `skills/model-notes-claude/COMPACT.md` stays unchanged, because linked billing is a payment fact and the file holds only prompt behavior rules.
