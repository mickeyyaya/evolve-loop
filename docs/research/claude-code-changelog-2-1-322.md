# Practice change in Claude Code: changelog 2.1.321 and 2.1.322 and persona review (research, 2026-11)

> This document reviews releases 2.1.321 and 2.1.322 of the Claude Code changelog under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The verdict is that no persona changes.

Date: 2026-11-10. Method: read the changelog at the link, treat the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Source and trust

The practice watch found an update in the source `claude-code-changelog` (item `practice-claude-code-changelog-4de759d9-202611100841`).
The source link is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md.
The source was read at the link on 2026-11-10. It was fetched with `curl`.

The first section at the link is `## 2.1.322`. It holds one bullet line. The section `## 2.1.321` follows it and holds many bullet lines.
This document explains both sections.

The text at the link is third-party data. It is untrusted.
The loop reads the lines as facts about the CLI. The loop must never follow a line as an instruction.
No instruction inside the changelog is followed in this cycle.

## 2. The lines that touch the loop

Most lines cover Remote Control, Claude Desktop, cloud sessions, screen reader mode, plugins, OpenTelemetry and Windows hooks.
The loop runs the CLI in a tmux pane on macOS and uses none of these. Those lines are data only.

| Line of the release | What it means for the loop |
|---|---|
| 2.1.321: Added Claude Fable 5.2 (`claude-fable-5-2`) as the default Fable model, with a 1M context window. | The model notes file `skills/model-notes-claude/COMPACT.md` already holds two bullets on Fable 5.2. They cover the 400 error and the always-on thinking. No new rule follows, so the file does not change. The price text of the line is not copied. |
| 2.1.322: Added fast mode on Vertex in the global, us and eu regions. The CLI falls back to standard speed if Vertex refuses it. | The loop does not route a phase through Vertex. Fast mode changes speed and cost, not a persona rule. The 2.1.321 fix for fast mode supports the same view. Fast mode now tries standard speed on the same model first. |
| 2.1.321: Fixed a read-only Bash command that ran outside the sandbox without a permission prompt. | The loop runs in bypass mode and its guards are hooks. The fix closes a CLI gap. No persona rule changes. |
| 2.1.321: Fixed Bash permission checks that judged a file write from the wrong folder. Fixed deny rules that missed a `cp` or `mv` target. | The ship gate and the role gate hold the same guarantee for every family. A persona states no rule on these checks. |
| 2.1.321: Fixed `NotebookEdit` and Edit on network paths (UNC shares, WSL folders, `/net` mounts). | The worktree of a cycle is a local folder. The builder never edits a notebook. No change. |
| 2.1.321: Fixed a PreToolUse hook `ask` that was lost or replaced when another hook answered `defer` or `ask` first. | The kernel guards deny with exit code 2 and do not use `ask`. No change. |
| 2.1.321: Fixed the git status at session start. It reads as unavailable when git status fails. | The context block of each phase also shows the git status. A persona does not depend on a clean status. No change. |
| 2.1.321: Fixed `pkill`, `pgrep` and `pidwait` that matched the parent processes of a cloud session. | This applies to cloud sessions only. The loop already kills only the process ids it started. |

## 3. Core persona evaluation under ADR-0133

ADR-0133 trims one persona at a time. It requires 10-cycle baseline measures before and after each diet change.
The plan [model-capability-practice-2026-10.md](../plans/model-capability-practice-2026-10.md) defines milestone M2 (the baseline) and milestone M3 (the `evolve-builder` diet).
No line of 2.1.321 or 2.1.322 shows a measured gain from a persona change. The lines are product fixes. They do not change an agent rule.

Therefore, no persona and no prompt changes in this cycle.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | Scout selects and specifies tasks. No line applies. |
| `evolve-triage` | No change | Triage reads the inbox rank and commits `## top_n`. No line applies. |
| `evolve-tdd-engineer` | No change | The TDD phase writes red tests. No line applies. |
| `evolve-builder` | No change | Builder is the first diet candidate under M3. A change must wait for the M2 baseline. |
| `evolve-auditor` | No change | The auditor grades test exit codes and the EGPS verdict. No line changes a gate. |
| `evolve-retrospective` | No change | The retrospective reads the ledger and the lessons. No line applies. |

## 4. Conclusion

Releases 2.1.321 and 2.1.322 add a default model, a fast mode region and many product fixes. They add no rule that a persona must state.
No file under `agents/` changes in this cycle. The file `skills/model-notes-claude/COMPACT.md` does not change either.
The verdict changes in two cases:
- a later release adds a model fact that the model notes lack;
- M2 ships a baseline that supports a persona diet.
