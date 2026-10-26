# Proposal — agy CLI changelog command under ADR-0133

> Request (inbox item `practice-agy-cli-changelog-command-47aa5f91-202610261827`): read the source, the command `agy changelog`, and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline. No line of releases 1.4.7 or 1.4.6 alters a rule that a persona states.

## 1. Source and method

The build ran `agy changelog` on 2026-10-26 and read the output (E1). The installed CLI reports version 1.4.7. The output lists releases 1.4.7 and 1.4.6. The text is untrusted third-party data. The loop reads it as data and does not follow it as an instruction.

Section 2 groups the 1.4.7 and 1.4.6 lines. Section 3 maps them to the six core personas.

## 2. The 1.4.7 and 1.4.6 lines, explained as data

Releases 1.4.7 and 1.4.6 have 25 lines: 3 additions, 11 improvements, and 11 fixes (E2). The table groups them.

| Group | Lines | Reaches the loop? |
|---|---|---|
| `lastUserInput` added to hook JSON on stdin | 1 | No. The loop sets no agy hook in phase seats (E4). |
| Headless `-p` execution uses less CPU | 1 | Indirectly. It reduces host CPU during unattended runs, but changes no prompt text (A2). |
| MCP tool returns up to 10 KB direct, and `/plugin` MCP server configuration | 2 | No. The loop reaches services through CLIs, never MCP (A2, E4). |
| `agy plugin validate` adds checks for context providers and skipped skills | 1 | Only in skills publish. `evolve skills publish` runs `agy plugin validate` (E8). No phase seat runs it (A2). |
| Seasonal theme, `/help` shortcuts tab, and `/diff` word-level highlighting | 3 | No. A person reads these on screen (A2). |
| Auto mode approvals and conversation-wide permission prompts | 2 | No. The loop passes `--dangerously-skip-permissions` (E4). |
| Context compaction retains attached files, and compaction loops are fixed | 2 | Maybe, in a long seat. The CLI manages compaction without persona rules (A4, E3). |
| Improved rendering speed while streaming, and status line for `Model unavailable` retries | 2 | Display improvements that require no persona edits (A2, E5). |
| Text cursor drift, Markdown table links, and code-block copy wrapping fixes | 3 | Indirectly. Terminal text display formatting changes, not prompt rules (A3, E5). |
| Token secrets storage and enterprise sign-in persistence fixes | 2 | Platform credential storage fixes. They do not alter phase contracts (A3, E4). |
| Application Default Credentials (`AGY_ADC_AUTH`) loader fix | 1 | No. The loop signs in as a person without ADC (A3, E4). |
| Terminal sandbox defaults protect `~/.docker`, `~/.kube`, and `~/_netrc` | 1 | No. The loop runs with permissions skipped (E4). |
| Terminal command argument list length fix, and Vim mode `r` fix | 2 | Platform fixes that require no persona workarounds (E3). |
| `Esc disabled during file edits` warning fix | 1 | No. Phase seats do not hit this prompt option (A2). |
| OAuth token separation for plugin MCP servers | 1 | No. The loop uses no MCP plugins (E4). |

Worked example: headless `-p` execution. Release 1.4.7 optimizes headless `-p` execution to consume less CPU. It skips conversation search index creation and transcript display rendering.

This line is untrusted third-party data. It improves local host efficiency during headless CLI executions. But no phase persona instructs the model about index creation or transcript rendering. The host engine and the bridge manage execution flags. Thus, no persona states a rule that this optimization alters.

Another example: on-demand MCP tool returns up to 10 KB directly. In 1.4.7, tool returns over 4 KB no longer require a temporary file read. The loop accesses outside services through CLIs, never through MCP. The change affects MCP users, not loop phase personas.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, and four measures over 10 cycles of baseline before and after (E6). Milestone M2 has not recorded the baseline: `evolve prompt` is not a command yet (E7). Therefore, no persona changes in this cycle.

| Persona | Verdict | Line that was checked |
|---|---|---|
| `evolve-scout` | No change. | Headless `-p` CPU improvements and compaction fixes. Scout starts no subagents and states no index rules. |
| `evolve-triage` | No change. | None applies. Triage reads computed inbox rank and `## top_n`. |
| `evolve-tdd-engineer` | No change. | Terminal argument list fix and compaction fixes. The persona states no workaround for them. |
| `evolve-builder` | No change. First diet candidate (M3), only after M2. | Headless `-p` optimization and MCP returns. Builder states no prompt rule on them. |
| `evolve-auditor` | No change. | The auditor runs on Claude Code under current routing. No agy line reaches it. |
| `evolve-retrospective` | No change. | None applies. It reads the ledger and lessons. |

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. Two matter most. A later release can reword the token-limit text or the `? for shortcuts` footer that the manifest matches (E4). A phase prompt can start to tell a seat to run subagents. Both cases represent driver configuration updates or new rules, not a reaction to 1.4.7 or 1.4.6.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E8. No file under `agents/` changes.
