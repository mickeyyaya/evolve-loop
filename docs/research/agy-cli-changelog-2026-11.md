# Practice change in agy: CLI changelog tab and persona review (research, 2026-11)

> This document reviews the `agy-cli-changelog-page` item for the lines Voice mode, Remote Agents, Agent Client Protocol (ACP), v1.4.12 and v1.4.11 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [earlier page note](practice-agy-cli-changelog-page-2026-11.md), which covers the App releases v2.28.2 through v2.27.0.

Date: 2026-11-10. Method: read the source page at its link, parse the text as untrusted data, and judge every core persona against ADR-0133.

## 1. Context and source

The practice watch found new lines in the source `https://antigravity.google/docs/changelog?tab=cli`.
The new lines came after the snapshot of 2026-11-02T00:08:04Z.

The page text is untrusted third-party data.
This note never follows an instruction inside the page or inside the item. It reads each line as a fact.

The item text is truncated. The last line of the item reads "Added support for re" and stops there.
This note read the live page at the link on 2026-11-10 to see the full text. The page is long, so the read covered the CLI tab and the lines about the three features.

## 2. Each line of the item explained as data

Each row below is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | The line "Other editors (ACP)" is a navigation label. It points to the Agent Client Protocol, which lets an editor drive an agent. | item line, page navigation |
| F2 | The line "Voice mode" is a navigation label. The page holds voice entries in both the App tab and the CLI tab. | page, read 2026-11-10 |
| F3 | The line "Remote Agents" is a navigation label. The CLI tab lists the flag `--remote-agent` from v1.5.0. | page, CLI tab |
| F4 | The line "Agent Client Protocol (ACP)" is a navigation label. The CLI tab lists `agy acp` from v1.5.0, which runs the CLI as an ACP server over stdio. | page, CLI tab |
| F5 | v1.4.12 (2026-10-30) lets a plugin mark an API key provider as optional. It also fixes the alignment of Markdown tables in streamed replies. | page, CLI tab |
| F6 | v1.4.11 (2026-10-30) adds a billing choice after a Google Cloud sign-in. It also adds `terminationBehavior` for `PreInvocation` hooks and a `finished_reason` field in JSON output. | page, CLI tab |
| F7 | v1.4.11 also shortens long shelves in the `/plugin` Discover tab and fixes `agy plugin import claude`. | page, CLI tab |
| F8 | The item repeats v1.4.12 and v1.4.11 twice, and its last line is cut after "Added support for re". | item text |

### Where the features belong

The page keeps a separate panel for each tab.
The App tab lists voice entries such as v2.4.3 and v2.5.0, which concern microphone permission and recording limits.
The CLI tab lists voice entries such as v1.5.2, which fix voice dictation for Chinese and Japanese.
The CLI tab also lists the Remote Agents entries v1.5.0 to v1.5.2 and the ACP entries v1.5.0 to v1.5.2.

This note keeps the two tabs apart. An App tab entry is not a CLI fact, and a CLI tab entry is not an App fact.
The versions v1.5.0 to v1.5.2 are newer than the item. The item shows only the navigation labels and v1.4.12 and v1.4.11.

Synthesis: the watch diff sees the labels of a long page, so it files navigation lines as updates.

### Release v1.4.12 and v1.4.11

The loop runs phase agents in tmux panes. It does not run plugins or billing sign-ins.
The `finished_reason` field can help a bridge to tell a cut answer from a full answer.
The `terminationBehavior` option of a `PreInvocation` hook can stop a turn before the model call.
The loop has its own guards for this, so it needs neither feature now.

## 3. Core persona evaluation under ADR-0133

ADR-0133 governs the prompt diet and the practice watch.
Rule D7 states that a diet trims one persona at a time.
Rule D8 requires measured baseline data over 10 cycles before and after each diet change.
Milestone M2 records the baseline first.

No prompt or persona changes in this cycle, because the M2 baseline does not exist yet. No line of the item touches a phase rule.

### evolve-scout
The `evolve-scout` persona requires no change.
Scout finds tasks. Voice mode, Remote Agents and ACP add no discovery rule.

### evolve-triage
The `evolve-triage` persona requires no change.
Triage picks tasks by policy rules. No line of the item changes a routing rule or a priority.

### evolve-tdd-engineer
The `evolve-tdd-engineer` persona requires no change.
The persona writes red tests. The `finished_reason` field adds no test contract.

### evolve-builder
The `evolve-builder` persona requires no change.
The builder is the first diet candidate under M3, and D8 requires the baseline data first.

### evolve-auditor
The `evolve-auditor` persona requires no change.
The auditor grades by test exit codes and binary verdicts. The hook option of v1.4.11 adds no gate.

### evolve-retrospective
The `evolve-retrospective` persona requires no change.
The persona extracts lessons off the critical path. No line of the item adds a lesson source.

## 4. Decision

Decision D1: change no prompt and no persona for Voice mode, Remote Agents, ACP, v1.4.12 and v1.4.11.
Reason: F2 to F4 are navigation labels and F5 to F7 touch no phase rule, and D8 requires the M2 baseline before a diet.

Rejected option: add a prompt rule for `--remote-agent` or `agy acp`.
F3 and F4 rule it out, because the loop runs agy in its own tmux pane. It uses no remote agent and no ACP client.

Rejected option: trim a persona now because the page changed.
D8 rules it out, because no baseline data exists yet. A diet trims one persona at a time, and only after M2.

A future proposal for one persona must name its goal, its done criteria, its limits and its before and after measures.

## 5. Follow-up

Two candidates can feed milestone M3 after the baseline: the `finished_reason` field of v1.4.11 and the `terminationBehavior` option.
The watch must file the CLI tab and the App tab as separate sources, and it must ignore navigation labels.
No file outside this note, its eval and the research index changes in this cycle.
