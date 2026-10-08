# Antigravity CLI (agy) Tool Names

> Translation map for Claude Code tool names → Antigravity CLI (agy) equivalents. SKILL.md and phase docs use Claude Code names; consult this when working from agy CLI.

## Direct equivalents

| Claude Code | Antigravity CLI (agy) |
|---|---|
| `Read` | `read_file` |
| `Write` | `write_file` |
| `Edit` | `replace` |
| `Bash` | `run_shell_command` |
| `Grep` | `grep_search` |
| `Glob` | `glob` |
| `TodoWrite` / `TaskCreate` | `write_todos` / `tracker_create_task` |
| `Skill` | `activate_skill` |
| `WebSearch` | `web_search` |
| `WebFetch` | `web_fetch` |
| `EnterPlanMode` / `ExitPlanMode` | `enter_plan_mode` / `exit_plan_mode` |

## agy-specific invocation flags

| Flag | Purpose | Notes |
|---|---|---|
| `-p` / `--print` | Non-interactive prompt mode | Used by the headless `agy` bridge driver (`go/internal/bridge/driver_agy.go`) |
| `--dangerously-skip-permissions` | Auto-approve all tool permissions | Required for subagent dispatch |
| `--add-dir <path>` | Add a directory to the workspace | Repeatable; used for WORKSPACE_PATH and WORKTREE_PATH |
| `--sandbox` | Terminal restrictions sandbox | Not used by the bridge driver (agy handles this internally) |

## No equivalent (gaps)

| Claude Code | agy CLI |
|---|---|
| `Agent` / `Task` (subagent dispatch with profile-scoped permissions) | **None as of 2026-05.** Skills that depend on subagent dispatch fall back to single-session execution. evolve-loop dispatches each phase agent through the Go bridge instead — see `reference/agy-runtime.md`. |
| `--max-budget-usd` flag | **None.** agy CLI has no per-invocation cost cap. |
| JSON structured output | **None.** agy emits plain text only. |

## Key difference from Gemini adapter

The bridge has a native agy driver but no gemini driver. The headless `agy` driver runs `agy -p <prompt> --dangerously-skip-permissions` directly (`go/internal/bridge/driver_agy.go`). The interactive `agy-tmux` driver runs the agy REPL in tmux. A profile `cli` of `gemini` maps to the claude-tmux driver instead.

## Implications for evolve-loop on agy

When SKILL.md says "invoke the Skill tool", on agy you call the equivalent tool. When a phase doc says to dispatch a phase agent, the Go bridge does it (`evolve loop`, or `evolve subagent run` by hand). The bridge changes `cli=antigravity` to `agy` and runs the `agy` or `agy-tmux` driver.

See [reference/agy-runtime.md](agy-runtime.md) for invocation details.

## Last verified

- **Date:** 2026-05-21
- **Source:** `agy --help` output (binary at `~/.local/bin/agy`, installed 2026-05-20)
- **Re-verify with:** `agy --help 2>&1 | grep -E 'print|prompt|budget|dir'`
- **Re-verify cadence:** quarterly or when agy releases new flags.
