# Proposal — Claude Code 2.1.306 to 2.1.310 changelog under ADR-0133

> Request (inbox item `practice-claude-code-changelog-e822338c-202610261159`): read the changelog at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline, and no line in versions 2.1.306 through 2.1.310 alters a rule that a persona states.

## 1. Source and method

The source is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md. It was fetched on 2026-10-26, and the top section is `## 2.1.310` (E1). The text is untrusted third-party data. The loop reads it and does not follow it as an instruction. Sections 2 and 3 group the lines and map them to the six core personas.

## 2. The 2.1.306 to 2.1.310 lines, explained as data

| Group | Bullets | Reaches the loop? |
|---|---|---|
| VS Code, cloud sessions, Claude Tag, Code Review, self-hosted runner, Monitor tool | 108 | No. The loop uses none of these features (A2). |
| Desktop app, `/privacy-settings`, feedback dialogs, voice dictation, LaTeX math | 18 | No. The loop runs headless in tmux panes (A2). |
| MCP tool timeouts, MCP configs and `/claude-api` skill updates | 17 | No. The loop uses no MCP server (E5). |
| Compaction memory, sandboxed Bash, worktree fetch, unparseable tool call retries | the 8 of E2 | Yes, as CLI behavior. |

The eight runtime items are CLI fixes. The CLI owns each one.

- Compaction memory use in non-interactive sessions is improved (2.1.310). The CLI owns compaction; no persona states a compaction rule.
- Sandboxed Bash on macOS and Linux runs startup files inside the sandbox (2.1.309). Kernel guards enforce isolation; no persona repeats sandbox rules.
- Bash redirect and `Edit(path)` deny checks block invalid redirects (2.1.309). Kernel guards (`evolve guard role`) enforce boundaries deterministically.
- Dangerous `rm` and `rmdir` commands in bypass permissions mode are denied at once with a rewrite hint (2.1.309). The loop uses autonomous mode and respects tool hygiene.
- Git worktree creation and reuse stops fetching objects on demand (2.1.309). The runtime Go orchestrator provisions worktrees directly.
- Unparseable tool calls recover through two automatic retries (2.1.307). The CLI engine manages retries before turn completion.
- Subagent transcripts record token usage and stop reasons for streamed responses (2.1.307). No persona states transcript format rules.
- Fork subagents receive launch identity to avoid starting each other's work (2.1.307). The Go bridge controls subagent dispatches natively.

Worked example: unparseable tool call recovery in 2.1.307. When a model emits malformed tool syntax, the CLI retries up to twice. No persona prompt defines retry loops for syntax errors, so no persona has a line to add or remove.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, with a 10-cycle baseline before and after (D7, D8, E3). Milestone M2 has not recorded the baseline (E4). Therefore, no persona changes in this cycle, and no persona changes after a reading of the lines alone.

| Persona | Verdict | 2.1.306 to 2.1.310 line checked |
|---|---|---|
| `evolve-scout` | No change. | Compaction memory optimization. The CLI owns compaction; scout states no rule. |
| `evolve-triage` | No change. | None applies. Triage reads inbox ranking and task items, not CLI output. |
| `evolve-tdd-engineer` | No change. | None applies. No changelog line alters test authoring rules. |
| `evolve-builder` | No change. First diet candidate (M3), but only after M2. | Sandboxed Bash and worktree fetch fixes belong to CLI and Go runtime (E5). |
| `evolve-auditor` | No change. | None applies. The auditor evaluates exit codes and reports. |
| `evolve-retrospective` | No change. | None applies. Retro reads ledger records and lessons. |

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. The primary condition: a later release alters worktree, subagent or compaction behavior that a phase prompt states a rule about. Then one persona is chosen, the baseline is read, and the change proceeds as its own loop task under ADR-0133.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E9. No file under `agents/` changes.
