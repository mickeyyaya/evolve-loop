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
| `-p` / `--print` | Non-interactive prompt mode | Used by `agy.sh` NATIVE adapter |
| `--dangerously-skip-permissions` | Auto-approve all tool permissions | Required for subagent dispatch |
| `--add-dir <path>` | Add a directory to the workspace | Repeatable; used for WORKSPACE_PATH and WORKTREE_PATH |
| `--sandbox` | Terminal restrictions sandbox | Not used by adapter (agy handles this internally) |

## No equivalent (gaps)

| Claude Code | agy CLI |
|---|---|
| `Agent` / `Task` (subagent dispatch with profile-scoped permissions) | **None as of 2026-05.** Skills that depend on subagent dispatch fall back to single-session execution. evolve-loop sidesteps this via the hybrid driver — see `reference/agy-runtime.md`. |
| `--max-budget-usd` flag | **None.** agy CLI has no per-invocation cost cap. Budget tracking is deferred (cost_blind:true in this infrastructure cycle). |
| JSON structured output | **None.** agy emits plain text only. The adapter appends a zero-cost envelope as the last STDOUT_LOG line. |

## Key difference from Gemini adapter

Unlike the gemini adapter, `agy.sh`'s NATIVE mode does invoke the binary directly (`agy -p`) because agy supports `--print` / `-p` non-interactive mode. No JSON translation is needed — agy emits plain text, and the adapter appends a hardcoded zero-cost envelope. The gemini adapter's JSON stats translation block does NOT port to agy.

## Implications for evolve-loop on agy

When SKILL.md says "invoke the Skill tool", on agy you call the equivalent tool. When a phase doc says "spawn via subagent-run.sh", that works directly — `subagent-run.sh` resolves `cli=antigravity → agy.sh` adapter, which invokes `agy -p` as a subprocess (NATIVE) or delegates to `claude -p` (HYBRID).

See [reference/agy-runtime.md](agy-runtime.md) for invocation details.

## Last verified

- **Date:** 2026-05-21
- **Source:** `agy --help` output (binary at `~/.local/bin/agy`, installed 2026-05-20)
- **Re-verify with:** `agy --help 2>&1 | grep -E 'print|prompt|budget|dir'`
- **Re-verify cadence:** quarterly or when agy releases new flags.
