# Practice change in Claude Code: changelog 2.1.317 and persona review (research, 2026-11)

> This document reviews release 2.1.317 of the Claude Code changelog under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The verdict is that no persona changes.

Date: 2026-11-10. Method: read the changelog at the link, treat the text as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Source and trust

The practice watch found an update in the source `claude-code-changelog`.
The source link is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md.
The file was fetched with `curl` on 2026-11-10. It has 10247 lines.

The first section at the link is now `## 2.1.321`. The sections `2.1.319`, `2.1.318` and `2.1.317` follow it.
The section `## 2.1.317` starts at line 360 and holds 87 bullet lines. This document explains that section.
The newer sections are outside the scope of this item. The next watch change files them.

The text at the link is third-party data. It is untrusted.
The loop reads the lines as facts about the CLI and never follows them as instructions.
The item text in the inbox is cut after about 35 lines. The full section at the link is the source of record.

## 2. The lines of 2.1.317 that touch the loop

Most lines cover cloud sessions, the Claude apps gateway, Claude Desktop, VS Code, Remote Control, Claude Tag and the Windows shell.
The loop runs the CLI in a tmux pane on macOS and uses none of these. Those lines are data only.

| Line of the release | What it means for the loop |
|---|---|
| Added `CLAUDE_CODE_DISABLE_NESTED_SESSION_ID`. Value `1` makes `claude -p` inside a session use that session's ID and transcript. | Watch item. Since this release, a `claude -p` call inside a session gets its own session ID. The loop runs `claude -p` probes in `go/internal/bridge/driver.go:31` and `go/internal/bridge/wallcorroborate.go:19`. Each probe now writes its own transcript and does not touch the transcript of a phase pane. The loop reads no control value from the environment, so it does not set the variable. |
| Fixed `claude -p` exiting 0 when a full disk or quota lost its transcript. It now prints an error and exits 1 in text and json output. | Watch item. A probe on a full disk now fails with exit 1 where it passed before. This is a correct signal. The CLI health check must report it as a host disk fault, not as a CLI fault. The disk guard of ADR-0133 decision 7 covers the cause. |
| Changed subagents and teammates on a different model to use that model's saved effort level or its default. | Watch item. The loop sets the effort of each phase through routing (`--effort` in `go/internal/bridge/launch.go`). Phases run in their own panes, not as subagents. The review subagents of the pre-commit fleet can see a changed effort. The effect is on cost, not on correctness. |
| Fixed a project's `effortLevel` setting ignored by pre-started sessions. | Applies to the desktop app only. The loop does not pre-start sessions. |
| Removed the Read tool's "file unchanged since your last Read" stub. | A re-read of an unchanged file now returns the content. No persona states a rule on this stub. A persona that says "read once" stays valid and costs a little more in tokens. |
| Added `onFailure: "block"` for Agent SDK hook callbacks. | The kernel guards are shell hooks, not SDK callbacks. They already block on exit code 2. No change. |
| Added a notice when auto mode's check has no answer for a subagent's tool call. | The loop runs in bypass mode. No change. |
| Fixed `claude auth status` exiting before a slow token refresh was saved. | The loop does not call `claude auth status` in a phase. No persona changes. |
| Fixed self-paced `/loop` wakeups not being scheduled when a field was missing. | This is the CLI `/loop` skill. The loop of this repository is `evolve loop`, a different thing. No change. |
| Fixed child processes of a stopped shell command sent SIGTERM twice. | This helps cleanup of a phase pane. The loop kills only the process ids it started, so no rule changes. |
| Fixed "Prompt is too long" after compaction with a very large `CLAUDE.md`, and very long conversations losing their earlier summary. | Both are CLI fixes. They support the context-size goal of ADR-0133 but do not change a persona rule. |
| Fixed `ultrathink` not raising effort in VS Code when telemetry is off. | The VS Code extension is not used. The `ultrathink` of `agents/evolve-builder.md` is a loop strategy name. No change. |

The other bullets are data only. They are the OTel attributes, the gateway cost report, the IdP group record, the Microsoft Foundry error, the `/design-sync` text and the plugin lines.

## 3. Core persona evaluation under ADR-0133

ADR-0133 trims one persona at a time and requires 10-cycle baseline measures before and after each diet change.
The plan [model-capability-practice-2026-10.md](../plans/model-capability-practice-2026-10.md) defines milestone M2 (the baseline) and milestone M3 (the `evolve-builder` diet).
On 2026-11-10, `./go/bin/evolve prompt stats` prints `unknown command "prompt"`, so M2 has not shipped.
No line of 2.1.317 shows a measured gain from a persona change. Therefore, no prompt and no persona changes in this cycle.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | Scout selects and specifies tasks. No line applies. |
| `evolve-triage` | No change | Triage reads the inbox rank and commits `## top_n`. No line applies. |
| `evolve-tdd-engineer` | No change | The TDD phase writes red tests. No line applies. |
| `evolve-builder` | No change | Builder is the first diet candidate under M3. A change must wait for the M2 baseline. The removed Read stub does not change its rules. |
| `evolve-auditor` | No change | The auditor grades test exit codes and the EGPS verdict. No line changes a gate. |
| `evolve-retrospective` | No change | The retrospective reads the ledger and the lessons. No line applies. |

## 4. Conclusion

Release 2.1.317 holds new settings, fixes and a few changes. It adds no rule that a persona must state.
No persona files change under `agents/` in this cycle.
The two watch items are the nested session ID line and the `claude -p` exit-code line. Both touch the `claude -p` probes of the bridge, not a persona.
The verdict changes in three cases:
- a probe starts to share a session transcript;
- a probe failure on a full disk is read as a CLI fault;
- M2 ships a baseline that supports a persona diet.
