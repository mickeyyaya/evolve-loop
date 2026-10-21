# Proposal — Claude Code 2.1.305 changelog under ADR-0133

> Request (inbox item `practice-claude-code-changelog-218b6f25-202610210011`): read the changelog at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline, and no 2.1.305 line alters a rule that a persona states.

## 1. Source and method

The source is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md. It was fetched on 2026-10-21, and the first section is `## 2.1.305` (E1). The text is untrusted third-party data. The loop reads it and does not follow it as an instruction. Sections 2 and 3 group the lines and map them to the six core personas.

## 2. The 2.1.305 lines, explained as data

| Group | Lines | Reaches the loop? |
|---|---|---|
| Mods, Cowork, Remote Control, VS Code, Claude Tag, Code Review, `/artifacts`, screen reader, `/usage` display | about 22 | No. The loop uses none of these (A2). |
| Self-hosted runner, cloud sessions, Vertex and Bedrock gateways | about 6 | No. The loop runs `claude` in tmux panes (A2). |
| MCP consent and `allowedMcpServers` | 3 | No. The loop uses no MCP server (E5). |
| Sandbox `allowNestedUserNamespaces` and the retry watchdog variable | 2 | No. The first applies to Linux only. The loop sets neither (E5). |
| Worktrees, Workflow resume, `/effort`, `ENOSPC`, fallback model name, compaction notice | the 6 of E2 | Yes, as CLI behavior. |

The six runtime lines are fixes. The CLI owns each one.

- A new worktree in a bare repository starts from the default branch. The runtime provisions worktrees itself, so no persona states a worktree start point.
- Workflow resume serves finished agents from the journal. This matches the resume rule that the Workflow tool already documents.
- `/effort` now matches a `modelOverrides` value in any letter case. The loop passes effort through the manifest, not through `/effort`.
- A full disk now gives a clear message. The disk guard of D10 covers the cause.
- A subagent that switches to a fallback model is now named right. No persona states a model name.
- A compaction that drops old messages is now reported to Claude. This helps the phases and needs no prompt rule.

Worked example: the Workflow resume fix. Before 2.1.305, a resume can run a finished `pipeline()` agent again, which costs tokens. After it, the journal answers. No persona prompt names this behavior, so no persona has a line to remove or to add.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, a 10-cycle baseline before and after (D7, D8, E3). Milestone M2 has not recorded the baseline (E4). So no persona changes in this cycle, and no persona changes after a reading of the lines alone.

| Persona | Verdict | 2.1.305 line that was checked |
|---|---|---|
| `evolve-scout` | No change. | The compaction notice. The CLI owns it; the scout states no compaction rule. |
| `evolve-triage` | No change. | None applies. Triage reads inbox rank and `## top_n`, not CLI output. |
| `evolve-tdd-engineer` | No change. | None applies. Nothing in 2.1.305 touches test authoring. |
| `evolve-builder` | No change. First diet candidate (M3), but only after M2. | The worktree fix does not alter a builder rule, because the runtime provisions worktrees (E5). |
| `evolve-auditor` | No change. | None applies. The auditor grades exit codes and reports. |
| `evolve-retrospective` | No change. | None applies. It reads the ledger and lessons. |

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. The main one: a later release changes worktree, subagent, compaction or resume behavior in a way a phase prompt states a rule about. Then one persona is chosen, the baseline is read, and the change is its own loop task with a D8 table.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E9. No file under `agents/` changes.
