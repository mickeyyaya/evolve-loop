# Gemini CLI Tool Names

> Translation map for Claude Code tool names → Gemini CLI equivalents. SKILL.md and phase docs use Claude Code names; consult this when working from Gemini CLI.

## Direct equivalents

| Claude Code | Gemini CLI |
|---|---|
| `Read` | `read_file` |
| `Write` | `write_file` |
| `Edit` | `replace` |
| `Bash` | `run_shell_command` |
| `Grep` | `grep_search` |
| `Glob` | `glob` |
| `TodoWrite` / `TaskCreate` | `write_todos` / `tracker_create_task` |
| `Skill` | `activate_skill` |
| `WebSearch` | `google_web_search` |
| `WebFetch` | `web_fetch` |
| `EnterPlanMode` / `ExitPlanMode` | `enter_plan_mode` / `exit_plan_mode` |

## No equivalent (gaps)

| Claude Code | Gemini CLI |
|---|---|
| `Agent` / `Task` (subagent dispatch with profile-scoped permissions) | **None.** Skills that depend on subagent dispatch fall back to single-session execution. evolve-loop sidesteps this via the hybrid driver — see `reference/gemini-runtime.md`. |
| `--max-budget-usd` flag | **None.** Gemini CLI has no per-invocation cost cap. Budget tracking must be external. |

## Gemini-only tools (no Claude Code equivalent)

| Gemini CLI | Purpose |
|---|---|
| `list_directory` | Enumerate files and subdirectories |
| `save_memory` | Persist facts to GEMINI.md across sessions |
| `ask_user` | Request structured input from the user |

## Implications for evolve-loop on Gemini

When SKILL.md says "invoke the Skill tool", on Gemini you call `activate_skill`. When a phase doc says "use Bash to run X", on Gemini you call `run_shell_command`. The semantic intent is identical.

When SKILL.md or any phase doc says "spawn via subagent-run.sh" or "the in-process Agent tool is forbidden", that sentence applies on Gemini too: you don't try to invent a Gemini subagent — you invoke the same shell script (`bash scripts/dispatch/subagent-run.sh ...`), which dispatches to the hybrid `gemini.sh` adapter, which spawns a real `claude -p` subprocess. The subagent isolation comes from the underlying Claude binary, not from Gemini.

See [reference/gemini-runtime.md](gemini-runtime.md) for invocation details.

## Last verified

- **Date:** 2026-04-30
- **Source:** `~/.claude/plugins/cache/claude-plugins-official/superpowers/5.0.7/skills/using-superpowers/references/gemini-tools.md`, plus 2026-03 forgery incident report
- **Re-verify with:** `gemini --help 2>&1 | grep -E 'tools|prompt|budget'` (capability surface) and the `~/.claude/plugins/.../using-superpowers/references/gemini-tools.md` upstream
- **Re-verify cadence:** quarterly. If Gemini CLI ships `gemini -p` (non-interactive prompt mode) or a `Task`-equivalent subagent dispatcher, this table needs updates and the hybrid-driver caveat in `gemini-runtime.md` may be relaxable.
