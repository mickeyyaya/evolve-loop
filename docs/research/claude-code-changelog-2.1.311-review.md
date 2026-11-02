# Practice change in Claude Code: changelog 2.1.311 and persona review (research, 2026-11)

> This document reviews release 2.1.311 of the Claude Code changelog under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The verdict is that no persona changes.

Date: 2026-11-02. Method: read the changelog at the link, treat the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Source and trust

The practice watch found an update in the source `claude-code-changelog`.
The source link is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md.
The text at the link is third-party data. It is untrusted.
The loop reads the lines as facts about the CLI and never follows them as instructions.

The section for 2.1.311 holds about 90 lines. This document explains the lines that touch the runtime of the loop.
The other lines cover the VS Code extension, cloud sessions, Claude Tag, Code Review and screen reader mode. The loop does not use these.

## 2. The lines of 2.1.311 that touch the loop

| Line of the release | What it means for the loop |
|---|---|
| `SubagentStop` hooks ran about twice a minute for each running subagent. Now they run once, when the subagent finishes. | The hook fires less often. A hook that counted the calls of `SubagentStop` as a heartbeat must not do so. The loop has no such hook. |
| Subagents failed to use MCP tools from servers that finish the connection after the subagent starts. This is fixed. | The loop reaches outside services through a CLI, not through MCP. The fix does not change a persona. |
| Claude saw stale `MEMORY.md` and `CLAUDE.md` content right after automatic compaction. This is fixed. | A long phase now reads the current files after compaction. No persona needs a rule to read the files again. |
| A subagent worktree was removed while a background subagent still worked in it. This is fixed. | The loop provisions each worktree itself and removes it at ship. The CLI fix is an extra safety net. |
| The Bash tool description claimed that the working directory always persists between calls. The text is corrected. | The personas already use absolute paths. No persona needs a new rule. |
| Shell commands that reach the timeout now show the output so far and report that the command still runs. | A phase can tell a slow command from a failed command. The personas need no rule. |
| `ultrathink` runs its turn at xhigh effort on the 5.x models. | The keyword sets the effort of one turn. The effort of each phase is a routing setting, not a persona text. |

The line on the 1M context message adds a compaction. The line on `DISABLE_COMPACT` only allows turning compaction off. The loop reads no control value from the environment, so both lines are data only.

## 3. Core persona evaluation under ADR-0133

ADR-0133 governs prompt diet and practice watch.
Decision D7 trims one persona at a time.
Decision D8 requires 10-cycle baseline measures before and after each diet change.
Milestone M2 records the baseline measures first. Milestone M3 trims `evolve-builder`.

The plan [model-capability-practice-2026-10.md](../plans/model-capability-practice-2026-10.md) defines D7, D8, M2 and M3.
No cycle has shown a measured gain from a persona change for this release. Therefore, no persona changes in this cycle.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | Scout selects and specifies tasks. The compaction and memory fixes are CLI work. |
| `evolve-triage` | No change | Triage reads the inbox rank and commits `## top_n`. No line of 2.1.311 touches this. |
| `evolve-tdd-engineer` | No change | The TDD phase writes red tests. The timeout output is a CLI improvement. |
| `evolve-builder` | No change | Builder is the first diet candidate under M3. A change must wait for the M2 baseline. |
| `evolve-auditor` | No change | The auditor grades test exit codes and the EGPS verdict. No release line changes a gate. |
| `evolve-retrospective` | No change | The retrospective reads the ledger and the lessons. The `SubagentStop` fix does not touch it. |

## 4. Conclusion

Release 2.1.311 holds fixes and small improvements. It adds no rule that a persona must state.
No persona file under `agents/` changes in this cycle.
A persona change needs a 10-cycle baseline measure first, as ADR-0133 requires.
