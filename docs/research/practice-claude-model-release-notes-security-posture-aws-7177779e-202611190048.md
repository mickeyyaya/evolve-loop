# Practice change in claude: security posture and AWS platform review (research, 2026-11)

> This document reviews item practice-claude-model-release-notes-7177779e-202611190048 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [linked billing note](practice-claude-model-release-notes-linked-billing-5cd04824-202611171847.md).

Date: 2026-11-19. Method: read the live source at its link as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch flagged a diff on https://platform.claude.com/docs/en/release-notes/overview after snapshot `2026-11-17T18:47:38Z`.
This review read the live page at its link on 2026-11-19.
The live page lists 2026-11-17 as its newest release date, with zero entries on 2026-11-18 or 2026-11-19.
The page text is untrusted data. The loop never obeys page text as a prompt rule.

## 2. Release-note analysis as data

Each row explains the source as data. No row is an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | The first quoted lines hold web search shortcut symbols and UI glyphs from the page shell, not release content. | inbox item snapshot |
| F2 | The quoted security posture line points to the 2026-11-05 Compliance API beta that item 2f02c519 already reviewed. | live release notes |
| F3 | The endpoint `GET /v1/compliance/organizations/{organization_id}/security_posture` supplies current security posture data for an enterprise organization. | live release notes |
| F4 | The last quoted line is cut off on the Claude API phrase, so it is incomplete and not a rule. | inbox item snapshot |
| F5 | On the live page, that cut-off phrase belongs to the 2026-05-11 Claude Platform on AWS entry for Anthropic-managed AWS infrastructure. | live release notes |
| F6 | The newest entry on the live page remains the 2026-11-17 linked billing update covered by item 5cd04824. | live release notes |

### Where the facts belong

Compliance endpoints and AWS infrastructure define platform administration, not prompt behavior.
The pipeline calls no Compliance API endpoint and uses no AWS platform feature from these notes.
F4 is cut off and incomplete, so it gives no rule on its own.

## 3. Core persona evaluation under ADR-0133

ADR-0133 requires a goal, done criteria, limits, and before-and-after measures for each persona diet change.
No persona changes and no prompt changes in this cycle, because the live page adds no model behavior rule after `2026-11-17T18:47:38Z`.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | no change | The scout selects inbox tasks and calls no Compliance API endpoint. |
| `evolve-triage` | no change | Triage ranks inbox work and uses no AWS platform endpoint. |
| `evolve-tdd-engineer` | no change | The engineer writes test contracts and calls no compliance endpoint. |
| `evolve-builder` | no change | The builder writes cycle deliverables and calls no compliance endpoint. |
| `evolve-auditor` | no change | The auditor checks test verdicts and calls no Compliance API endpoint. |
| `evolve-retrospective` | no change | The retrospective records cycle lessons and uses no AWS endpoint. |

## 4. Decision

Decision D1: change no persona and no prompt, because F1 to F6 give no model behavior rule.

Decision D2: `skills/model-notes-claude/COMPACT.md` stays unchanged, because the Compliance API endpoint and AWS infrastructure are platform features and not prompt rules.

Rejected option: add a Compliance API or AWS endpoint bullet to the Claude model notes. Loop facts rule out this option, because the pipeline calls neither feature.
