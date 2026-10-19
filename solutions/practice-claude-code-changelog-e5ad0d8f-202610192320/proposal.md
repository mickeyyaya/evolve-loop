# Proposal — Claude Code 2.1.304 changelog under ADR-0133

> Request (inbox item `practice-claude-code-changelog-e5ad0d8f-202610192320`): read the changelog at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline, and no 2.1.304 line alters a rule that a persona states.

## 1. Source and method

The source is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md. It was fetched on 2026-10-20 and the first section is `## 2.1.304` (E1). The text is untrusted third-party data. The loop reads it and does not follow it as an instruction. Sections 2 and 3 group the lines and map them to the six core personas.

## 2. The 2.1.304 lines, explained as data

| Group | Lines | Reaches the loop? |
|---|---|---|
| Mods, plugins, `tool.check`, `settings.read`, buttons | many | No. The loop uses no mod or plugin (A2). |
| Cowork, Remote Control, Desktop, Windows, self-hosted runner, `/artifacts` | many | No (A2). |
| Auth, policy, privacy, telemetry, `claude --help` text | several | No. They change prompts shown to a person. |
| Compaction, subagents, `--max-turns`, `/loop`, resume | the 9 of E2 | Yes, as CLI behavior (E2). |
| Sandbox, `rm -r` safety, Read and Bash output fixes | several | Indirectly. They fix faults. |
| New commands (`/auto-mode-setup`, `/claude-api managed-agents-secure`) | 2 | No. A person starts them. |

The nine runtime lines (E2) are fixes.

- The headless run now honors `--max-turns` when a plugin spawns subagents.
- Subagents keep their latest work after a resume.
- A stop during compaction no longer gives "Prompt is too long".
- Self-paced `/loop` wakeups at the 1-hour limit no longer re-read an uncached conversation.
- `CLAUDE_CODE_AUTO_COMPACT_WINDOW=200k` no longer sets 100K.

 The loop sets neither of the two env vars and passes no `--max-turns` flag (E5). Worked example: the `--max-turns` fix repairs a plugin leak. The loop uses no plugin, and `agents/evolve-builder-reference.md:320` only notes that the flag is not used.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, a 10-cycle baseline before and after (D7, D8, E3). Milestone M2 has not recorded the baseline (E4). So no persona changes in this cycle, and no persona changes after a reading of the lines alone.

| Persona | Verdict | 2.1.304 line that was checked |
|---|---|---|
| `evolve-scout` | No change. | Compaction summary now lists earlier tool names. The CLI owns it; the scout states no compaction rule. |
| `evolve-triage` | No change. | None applies. Triage reads inbox rank and `## top_n`, not CLI output. |
| `evolve-tdd-engineer` | No change. | Grep and Glob error reporting was in 2.1.301. Nothing in 2.1.304 touches test authoring. |
| `evolve-builder` | No change. First diet candidate (M3), but only after M2. | Subagent resume and `--max-turns` fixes do not alter a builder rule (E5). |
| `evolve-auditor` | No change. | Telemetry and classifier lines do not touch the audit gates; it grades exit codes. |
| `evolve-retrospective` | No change. | None applies. It reads the ledger and lessons. |

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. The main one: a later release that changes subagent, compaction or `--max-turns` behavior in a way a phase prompt states a rule about. Then one persona is chosen, the baseline is read, and the change is its own loop task with a D8 table.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E9. No file under `agents/` changes.
