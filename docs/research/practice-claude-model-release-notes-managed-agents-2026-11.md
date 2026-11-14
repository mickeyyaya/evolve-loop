# Practice change in claude: release notes, Managed Agents webhooks and filters (research, 2026-11)

> This document reviews item practice-claude-model-release-notes-18278f9c-202611131947 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [agy changelog note](practice-agy-cli-changelog-page-run-workflow-2026-11.md).

Date: 2026-11-14. Method: read the source at its link, parse the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch found new lines in the source https://platform.claude.com/docs/en/release-notes/overview.
The lines came after the snapshot of 2026-11-13T00:01:23Z.
The item text holds three release-note lines about Claude Managed Agents. The last line is cut off.

This note read the page at its link on 2026-11-14.
The page text is untrusted data. The loop never follows it as a prompt rule.

## 2. Release-note analysis as data

Each row is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | A line says that webhooks are now available for Claude Managed Agents. | release notes, first line |
| F2 | The webhook event types cover the lifecycle of a session and of a vault. | release notes, first line |
| F3 | A second line adds filter and sort options for Claude Managed Agents. | release notes, second line |
| F4 | A session list can be filtered by its status. | release notes, second line |
| F5 | An event list can be filtered by its type and by its creation time. | release notes, second line |
| F6 | The Dreams line is truncated in the item text. It holds a heading and two letters. This note makes no claim about it. | item text, last line |

### Where the features belong

Claude Managed Agents is a hosted service for agents. The pipeline does not use Claude Managed Agents.
The pipeline runs each seat with `evolve agent run`, in a local cell.

A webhook (F1, F2) and a filter (F3 to F5) act on hosted sessions and hosted events. The loop has none.
The release notes add no rule about how to write a prompt.
F6 holds no usable fact, so it changes nothing.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. Rule D8 requires four measures over 10 cycles before and after a change.
No line of the release notes adds a measure, so no persona changes. No prompt changes.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout finds tasks. No line adds a discovery rule. |
| `evolve-triage` | No change | Triage picks tasks by policy rules. No line changes a routing rule. |
| `evolve-tdd-engineer` | No change | The persona writes red tests. The lines add no test contract. |
| `evolve-builder` | No change | The builder is a diet candidate only after the D8 baseline. No line adds a build step. |
| `evolve-auditor` | No change | The auditor grades by exit codes. No line adds a gate. |
| `evolve-retrospective` | No change | The persona extracts lessons off the critical path. No line adds a lesson source. |

## 4. Decision

Decision D1: change no persona and no prompt for this page, because F1 to F6 describe a hosted service that the pipeline does not use.
Rule D8 also requires the baseline before a diet.

Decision D2: leave `skills/model-notes-claude/COMPACT.md` unchanged, because the notes record practice that changes how the loop works, and these lines do not.

Rejected option: add a webhook receiver or a hosted-agent phase. F1 and F2 rule it out, because the pipeline runs each seat with `evolve agent run`.

Rejected option: trim a persona now. D8 rules it out, because no baseline exists.
