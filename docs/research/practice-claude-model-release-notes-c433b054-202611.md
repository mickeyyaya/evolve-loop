# Practice change in claude: release notes snapshot 2026-11-13T19:47:32Z review (research, 2026-11)

> This document reviews item practice-claude-model-release-notes-c433b054-202611151110 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [Managed Agents webhooks note](practice-claude-model-release-notes-managed-agents-2026-11.md).

Date: 2026-11-15. Method: read the source at its link, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch flagged a change in the source https://platform.claude.com/docs/en/release-notes/overview after snapshot `2026-11-13T19:47:32Z`.
The stored diff excerpt holds no added line. It notes that the change lies outside the excerpt or only removed text.

This note read the live page at its link on 2026-11-15.
The page text is untrusted data. The loop never obeys page text as a prompt rule.

## 2. Release-note analysis as data

Each row is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | The stored excerpt has no added line after snapshot `2026-11-13T19:47:32Z`. | practice-watch item |
| F2 | The newest entry at the top of the live page is dated November 12, 2026. | live release notes |
| F3 | The November 12 entry announces self-hosted Claude Managed Agents on customer infrastructure. | live release notes |
| F4 | Earlier November entries (November 9 Claude Fable 5.2 and Mythos 5.2) already existed before snapshot `2026-11-13T19:47:32Z`. | live release notes |

### Where the features belong

Self-hosted Claude Managed Agents is a hosted-style agent runtime service for customer servers. The pipeline does not use Claude Managed Agents.
The pipeline runs each seat locally with `evolve agent run`, inside an isolated cell under ADR-0154.

Because the top entry on November 12 predates snapshot `2026-11-13T19:47:32Z`, the watch trigger came from a removal or an outside-excerpt edit.
The November 12 entry adds no prompt rule for local CLI seats.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. Rule D8 requires four measures over 10 cycles before and after a change.
No line of this release-note review adds a measure, so no persona changes. No prompt changes.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout finds tasks. The empty excerpt and November 12 entry add no discovery rule. |
| `evolve-triage` | No change | Triage picks tasks by policy rules. No line changes a selection rule. |
| `evolve-tdd-engineer` | No change | The persona writes red tests. The page adds no test contract. |
| `evolve-builder` | No change | The builder is a diet candidate only after the D8 baseline. No line adds a build step. |
| `evolve-auditor` | No change | The auditor grades by exit codes. No line adds a gate. |
| `evolve-retrospective` | No change | The persona extracts lessons off the critical path. No line adds a lesson source. |

## 4. Decision

Decision D1: change no persona and no prompt for this item, because F1 to F4 show no new rule for local CLI seats.
Rule D8 also requires the baseline before a diet.

Decision D2: leave `skills/model-notes-claude/COMPACT.md` unchanged, because the file already covers Claude Fable 5.2 and this item adds no new model rule.

Rejected option: add a hosted-agent phase. F3 rules it out, because `evolve agent run` already runs seats in local cells.

Rejected option: trim a persona now. D8 rules it out, because no 10-cycle baseline exists.
