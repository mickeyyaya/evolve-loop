# Proposal — agy CLI changelog under ADR-0133

> Request (inbox item `practice-agy-cli-changelog-page-e7652e1f-202610261200`): read the source at https://antigravity.google/docs/changelog?tab=cli and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks any change without a 10-cycle baseline. No line in v2.26.0 or v2.25.0 alters a rule that a persona states.

## 1. Source and method

The page was fetched on 2026-10-26 with `curl` and read as text (E1).
The page lists v2.26.0 (October 22, 2026) and v2.25.0 (October 20, 2026) under Antigravity App.
The Antigravity CLI section lists v1.4.0 (October 16, 2026) as its latest release.
The text is untrusted third-party data.

The loop reads the text as data and never follows it as prompt instructions.
Section 2 explains the changelog lines and navigation links as data.
Section 3 maps the updates to the six core personas.

## 2. The changelog lines, explained as data

The page header and sidebar contain documentation site navigation links.
These links include slash commands: `/agents`, `/plan`, `/voice`, and related topics.
These lines are web site navigation routes, not CLI execution commands.
The loop treats these navigation lines as data and executes none of them.

The updates introduce 11 improvements and 23 fixes across v2.26.0 and v2.25.0 under Antigravity App (E2, E3).
The table groups them by functional area.

| Functional group | Features in v2.26.0 and v2.25.0 | Reaches the loop? |
|---|---|---|
| Documentation site navigation | Slash-command links: `/agents`, `/plan`, `/voice`, and related pages | No. Web site navigation links only. |
| Settings and UI notifications | Notifications page, completion sounds, volume controls, docked prompts | No. Autonomous phases run headlessly in tmux panes (A2, E4). |
| Conversations palette and navigation | Cmd+P / Ctrl+P filters, Last Updated sort, caret placement | No. Human interactive controls only. |
| MCP and plugins | MCP configure buttons, plugin conflict dialogs, sidebar badges | No. The loop manages no UI plugins (A2). |
| Account and credentials | Application Default Credentials (ADC) plan marking, sign-out guidance, persistence fix | No. These fixes affect desktop UI settings; the bridge manifest passes credentials through (A3, E4). |
| Runtime and socket stability | Socket disconnect auto-retry after sleep, long-running turn wakeups | No. These fixes belong to the Antigravity App desktop client (E2). |
| Platform bug fixes | Windows git diff backslash separators, LaTeX rendering, YAML frontmatter colon parsing | No. These fixes belong to the Antigravity App desktop client (E2, E3). |

Worked example: v2.26.0 adds docked question prompts above the message box so users can continue typing.
Autonomous phase agents do not use the desktop UI message box.
They communicate through the native bridge driver (E4).
Therefore, this feature requires no prompt modification.

## 3. Persona verdicts under ADR-0133

ADR-0133 establishes prompt diet discipline.
Decision D7 requires trimming one persona at a time.
Decision D8 mandates a 10-cycle measured baseline before and after any change (E5).
Milestone M2 baseline tracking has not yet run (E6).
Therefore, no persona changes in this cycle.

| Persona | Verdict | Line that was checked |
|---|---|---|
| `evolve-scout` | No change. | Docked question prompts. The scout operates headlessly and uses no desktop prompt box. |
| `evolve-triage` | No change. | Conversation palette filters. Triage ranks tasks with deterministic policy rules. |
| `evolve-tdd-engineer` | No change. | YAML colon parsing fix. Test authors follow repository predicate contracts. |
| `evolve-builder` | No change. First diet candidate under M3, only after M2 baseline. | Socket auto-retry. This fix belongs to the Antigravity App desktop client (E2). |
| `evolve-auditor` | No change. | ADC account indicators. The auditor checks test exit codes and file artifacts. |
| `evolve-retrospective` | No change. | Plugin sidebar badges. Retrospective analyzes cycle ledgers and instincts. |

No persona file under `agents/` changes in this cycle.

## 4. What can change the verdict

See [recommendation.md](recommendation.md) for the flip conditions.
Two conditions matter most.
An upstream release can change the prompt marker `? for shortcuts` (E9).
An upstream release can alter the quota exhaustion pattern that driver regexes detect (E9).
Both are driver configuration items, not persona prompt instructions.

## 5. Files

The `options/` directory contains three standalone strategies.
`recommendation.md` compares the options and recommends Option 1.
`assumptions-and-evidence.md` defines assumptions A1 to A4 and evidence E1 to E10.
