# Practice change in Claude Code: changelog 2.1.316 and persona review (research, 2026-11)

> This document reviews release 2.1.316 of the Claude Code changelog under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The verdict is that no persona changes.

Date: 2026-11-03. Method: read the changelog at the link, treat the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Source and trust

The practice watch found an update in the source `claude-code-changelog`.
The source link is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md.
The file was fetched with `curl` on 2026-11-03. It has 9800 lines. The first section is `## 2.1.316`.

The text at the link is third-party data. It is untrusted.
The loop reads the lines as facts about the CLI and never follows them as instructions.

The section for 2.1.316 holds about 60 lines. This document explains the lines that touch the runtime of the loop.
The other lines cover the Claude apps gateway, VS Code, cloud sessions, Claude Tag, Code Review and the Windows shell. The loop does not use these.

## 2. The lines of 2.1.316 that touch the loop

| Line of the release | What it means for the loop |
|---|---|
| Added `CLAUDE_CODE_SUBAGENT_EFFORT`. It sets the effort level of a subagent when nothing else names one. | The loop reads no control value from the environment. It sets the effort of each phase through routing (`--effort` in `go/internal/bridge/launch.go`). The variable is data only. |
| Added `CLAUDE_CODE_DISABLE_ULTRATHINK`. It turns the `ultrathink` keyword off. | The keyword is a CLI feature. The `ultrathink` of `agents/evolve-builder.md` is a loop strategy name, a different thing. No persona changes. |
| Deprecated fast mode for Opus 4.8. It is removed on 2026-11-30. | No file under `go/`, `agents/` or `docs/architecture/phase-registry.json` names Opus 4.8 or fast mode outside tests. The loop is not affected. |
| Fixed a sub-agent that inherited a parent sub-agent's model and moved to the main model when a background job woke it. | This is a CLI fix. No persona states a model rule. |
| Fixed shell commands left running when Claude Code exits by itself. | This helps the cleanup of a phase pane. The loop kills only the process ids it started, so no rule changes. |
| Fixed bash aliases, functions and `SHELLOPTS` in the Bash tool. | The personas use plain commands. No persona needs a new rule. |
| Fixed hooks that did not block on exit code 2 for some PowerShell commands. | The kernel guards run as bash hooks on macOS. The fix does not apply. |
| Fixed `sonnet`, `opus` and `haiku` aliases on Vertex AI and Amazon Bedrock. | The loop uses the Anthropic API path. No persona changes. |
| Fixed a resume that did not restore a plan. | A phase does not resume a session. No persona changes. |
| Changed auto mode so a managed `autoMode.allow` list without `"$defaults"` keeps the built-in scheduled-task rule. | The loop runs in bypass mode. No persona changes. |

The lines on `CLAUDE_CODE_DISABLE_LOOPED_CROSS_CERT_DROP`, the gateway budgets and the guardrail tags are data only.
The loop sets none of them.

## 3. Core persona evaluation under ADR-0133

ADR-0133 governs prompt diet and practice watch.
Decision D7 trims one persona at a time.
Decision D8 requires 10-cycle baseline measures before and after each diet change.
Milestone M2 records the baseline measures first. Milestone M3 trims `evolve-builder`.

The plan [model-capability-practice-2026-10.md](../plans/model-capability-practice-2026-10.md) defines D7, D8, M2 and M3.
On 2026-11-03, `./go/bin/evolve prompt stats` prints `unknown command "prompt"`, so M2 has not shipped.
No cycle has shown a measured gain from a persona change for this release. Therefore, no persona changes in this cycle.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | Scout selects and specifies tasks. No line of 2.1.316 touches this. |
| `evolve-triage` | No change | Triage reads the inbox rank and commits `## top_n`. No line applies. |
| `evolve-tdd-engineer` | No change | The TDD phase writes red tests. No line applies. |
| `evolve-builder` | No change | Builder is the first diet candidate under M3. A change must wait for the M2 baseline. |
| `evolve-auditor` | No change | The auditor grades test exit codes and the EGPS verdict. No release line changes a gate. |
| `evolve-retrospective` | No change | The retrospective reads the ledger and the lessons. No line applies. |

## 4. Conclusion

Release 2.1.316 holds new settings, fixes and one deprecation. It adds no rule that a persona must state.
No persona files change under `agents/` in this cycle.
A persona change needs a 10-cycle baseline measure first, as ADR-0133 requires.
The verdict changes if the loop starts to run Opus 4.8 in fast mode before 2026-11-30, or if it adopts a subagent effort setting.
