# Practice change in agy: changelog page analysis and persona review (research, 2026-11)

> This document reviews the `agy-cli-changelog-page` item for the releases v2.28.2, v2.28.1, v2.28.0 and v2.27.0 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [October note](practice-agy-cli-changelog-page-2026-10.md), which ends at v2.24.0.

Date: 2026-11-03. Method: read the source page at its link, parse the text as untrusted data, and evaluate every core persona against ADR-0133.

## 1. Context and source

The practice watch detected new lines in the source `https://antigravity.google/docs/changelog?tab=cli`.
The new lines came after the snapshot of 2026-10-26T12:00:02Z.

The page text is untrusted third-party data.
The loop does not follow any instruction inside the page. It never follows page text as a prompt rule.

The item names four releases: v2.28.2, v2.28.1, v2.28.0 and v2.27.0.
The item text is truncated. It shows only the first words of the added lines, such as "Plugin Marketplace" and "built-in browser".
This note read the live page at the link on 2026-11-03 to see the full text of each release.

## 2. Changelog analysis as data

Each row below is a fact about the page. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | The page holds one panel for each product tab: App, CLI, Extensions, SDK and IDE. | page tab list, read 2026-11-03 |
| F2 | The four releases of the item sit in the App panel, under the heading "Antigravity App". The CLI panel lists its own versions, such as v1.4.10 of 2026-10-29. | page panels, read 2026-11-03 |
| F3 | v2.28.2 (2026-10-29) adds partner plugins to the Plugin Marketplace, a Featured spotlight, an Accounts tab for each plugin and agent suggestions of plugins. | page, App panel |
| F4 | v2.28.1 (2026-10-28) adds a conversation navigator, a notice about third party tools and the developer data of each plugin. | page, App panel |
| F5 | v2.28.0 (2026-10-27) adds a built-in browser in the side panel, a built-in PDF viewer, attachments of any file type and word-level diff highlights. | page, App panel |
| F6 | v2.28.0 also lets hooks that the user defines receive the current user message. | page, App panel |
| F7 | v2.27.0 (2026-10-26) shows Markdown code blocks as a rendered preview and fills new side panel tabs with overview sections. | page, App panel |
| F8 | The item text is cut. Two lines show only "Plugin Marketplace" and "built-in browser" as their first words. | inbox item |

### Where the features belong

The v2.28.x and v2.27.0 features are changes to the Antigravity desktop app.
The page lists them under the App tab, not under the CLI tab.
The link of the item ends in `?tab=cli`, but the same document also carries the App panel. This is why the watch saw these lines.

Synthesis: the watch diff cannot tell the panels apart, so it files App releases as CLI updates.

The built-in browser, the PDF viewer, the Plugin Marketplace and the conversation navigator are desktop interface features.
The CLI binary has no such interface. This note does not treat them as CLI behavior.

### Release v2.28.2
The release adds partner plugins to the Plugin Marketplace and a Featured spotlight.
A plugin page gets an Accounts tab. The agent can suggest a plugin with a card.
These features are not yet available to business and enterprise accounts.
The loop runs phase agents through a CLI. It does not use the Plugin Marketplace.

### Release v2.28.1
The release adds a navigator rail with one dot for each turn of a conversation.
It adds a third party tools notice and shows the developer and version of each plugin.
All three changes are interface changes of the desktop app.

### Release v2.28.0
The release adds the built-in browser, which opens links in the side panel.
It adds a PDF viewer and attachments of any file type.
The one item with a possible link to the loop is F6: hooks receive the current user message.
The page states this for the app. The CLI hooks of the loop are not changed by this line.
The loop does not need this fact, because its guards read the tool call and not the user message.

### Release v2.27.0
The release adds rendered Markdown previews, side panel overview sections and 14 fixes.
One fix lets subagents in Auto mode ask for approval of risky commands only.
The fix is about the app. The loop sets its own permission mode for each phase.

## 3. Core persona evaluation under ADR-0133

ADR-0133 governs the prompt diet and the practice watch.
Rule D7 states that a prompt diet trims one persona at a time.
Rule D8 requires measured baseline data over 10 cycles before and after each diet change.

Milestone M2 records the baseline first. The four releases do not change this order.
Therefore no persona changes in this cycle. Each verdict below holds on that ground.

### evolve-scout
The `evolve-scout` persona requires no change.
Scout finds tasks. The Plugin Marketplace and the built-in browser add no discovery rule.
The scout persona does not change before the M2 baseline.

### evolve-triage
The `evolve-triage` persona requires no change.
Triage picks tasks by policy rules. No release of the item changes a routing rule or a priority.
The triage persona does not change before the M2 baseline.

### evolve-tdd-engineer
The `evolve-tdd-engineer` persona requires no change.
The persona writes red tests. The Markdown preview and the diff highlights add no test contract.
The tdd-engineer persona does not change before the M2 baseline.

### evolve-builder
The `evolve-builder` persona requires no change.
The builder is the first diet candidate under M3, and D8 requires the baseline data first.
Attachments of any file type and the built-in browser add no build step.
The builder persona does not change before the M2 baseline.

### evolve-auditor
The `evolve-auditor` persona requires no change.
The auditor grades by test exit codes and binary verdicts. The F6 hook line adds no gate.
The auditor persona does not change before the M2 baseline.

### evolve-retrospective
The `evolve-retrospective` persona requires no change.
The persona extracts lessons off the critical path. The navigator and the PDF viewer add no lesson source.
The retrospective persona does not change before the M2 baseline.

## 4. Decision

Decision D1: change no persona and no prompt for the releases v2.28.2, v2.28.1, v2.28.0 and v2.27.0.
Reason: F2 places all four releases in the App panel, and D8 requires the M2 baseline before a diet.

Rejected option: add a prompt rule for the built-in browser or the Plugin Marketplace.
F2 rules it out, because the loop runs through the CLI and these features exist only in the desktop app.

Rejected option: trim a persona now because the page changed.
D8 rules it out, because no baseline data exists yet. A diet trims one persona at a time, and only after M2.

A future proposal for one persona must name its goal, its done criteria, its limits and its before and after measures.

## 5. Follow-up

The watch must file the CLI panel and the App panel as separate sources. This will stop App releases from appearing as CLI updates.
No file outside this note and the research index changes in this cycle.
