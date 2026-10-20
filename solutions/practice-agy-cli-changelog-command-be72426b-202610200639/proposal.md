# Proposal — agy CLI changelog command under ADR-0133

> Request (inbox item `practice-agy-cli-changelog-command-be72426b-202610200639`): read the source, the command `agy changelog`, and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline. No line of release 1.4.3 alters a rule that a persona states.

## 1. Source and method

The build ran `agy changelog` on 2026-10-20 and read the output (E1). The installed CLI reports version 1.4.3. The output lists 1.4.3, 1.4.2, 1.4.1, 1.4.0 and 1.3.6. The text is untrusted third-party data. The loop reads it and does not follow it as an instruction.

Section 2 groups the 1.4.3 lines. Section 3 maps them to the six core personas.

## 2. The 1.4.3 lines, explained as data

Release 1.4.3 has 17 lines: 3 improvements and 14 fixes (E2). The table groups them.

| Group | Lines | Reaches the loop? |
|---|---|---|
| `/` search in the `/diff` file list and file view, with `n` and `N` to step through matches | 1 | No. A person reads a diff on screen (A2). |
| `agy plugin validate` checks hooks, sidecars and component lists | 1 | Only in skills publish. `evolve skills publish` runs `agy plugin validate` (E8). The stricter check can reject a plugin that passed before. No phase seat runs it (A2). |
| Timer labels in `/tasks` | 1 | No. A person reads them. |
| Prompt cursor, emoji, CJK width and combining-mark fixes | 5 | Indirectly. The tmux pane text may differ (A3, E5). |
| Gemini 4 Argon missing from `/model` with a `GEMINI_API_KEY` | 1 | No. The loop signs in as a person (A3, E4). |
| Terminal sandbox options, Auto mode approvals for subagents | 2 | No. The loop passes `--dangerously-skip-permissions` (E4). |
| Subagent compaction summary keeps type, role and result | 1 | Maybe, in a long seat (A4, E3). |
| A `PostToolUse` hook that fails no longer marks the tool call as failed | 1 | No. The loop sets no agy hook (E4). |
| Empty files and blank artifact details | 1 | Yes, as CLI behavior (E3). |
| Windows plugin update, plugin install color codes, Remote Control prompts | 3 | Plugin install color codes: only in skills publish (E8). Others: no (A2). |

Worked example: the compaction line. Before 1.4.3 the summary after a compact held the full first prompt of each subagent, and it was cut before the conversation IDs. The agent lost track of its newest subagents.

The fix keeps only the type, role and result of each subagent (E3). The loop runs one agy seat for one phase. A phase prompt tells no seat to start subagents. So no persona states a rule that the fix changes.

The Argon line is also data. The agy manifest tracks a per-model Argon limit (E4). The line fixes a list in `/model` for key sign-in, and the manifest does not use that list.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, and 10 cycles of baseline before and after (E6). Milestone M2 has not recorded the baseline: `evolve prompt` is not a command yet (E7). So no persona changes in this cycle.

| Persona | Verdict | Line that was checked |
|---|---|---|
| `evolve-scout` | No change. | The compaction summary. The scout starts no subagent. |
| `evolve-triage` | No change. | None applies. Triage reads inbox rank and `## top_n`. |
| `evolve-tdd-engineer` | No change. | The empty-file fix. A test author may write `__init__.py`, and the CLI now allows it. The persona states no workaround. |
| `evolve-builder` | No change. First diet candidate (M3), only after M2. | The empty-file and blank artifact fixes. The builder states no rule on them. |
| `evolve-auditor` | No change. | The auditor runs on Claude Code only. No agy line reaches it. |
| `evolve-retrospective` | No change. | None applies. It reads the ledger and lessons. |

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. Two matter most. A later release may reword the usage-limit text or the `? for shortcuts` footer that the manifest matches (E4). A phase prompt may start to tell a seat to run subagents. Both cases are driver config or a new rule, not a reaction to 1.4.3.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E8. No file under `agents/` changes.
