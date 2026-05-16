# Claude Code Tool Names

> Canonical tool names used throughout SKILL.md and phase docs. This is the reference platform — the names below are what the rest of the documentation uses verbatim.

## File operations

| Tool | Purpose |
|---|---|
| `Read` | Read a file from disk |
| `Write` | Create or overwrite a file |
| `Edit` | Apply an exact-string replacement to an existing file |
| `Glob` | Match files by glob pattern |
| `Grep` | Content search with ripgrep |

## Shell

| Tool | Purpose |
|---|---|
| `Bash` | Run a shell command (subject to PreToolUse hook gating) |

## Web

| Tool | Purpose |
|---|---|
| `WebSearch` | Web search |
| `WebFetch` | Fetch and summarize a URL |

## Orchestration

| Tool | Purpose |
|---|---|
| `Skill` | Invoke a registered skill by name |
| `Agent` (a.k.a. `Task`) | Dispatch a subagent with profile-scoped permissions |
| `TaskCreate` / `TaskUpdate` / `TaskList` | Track in-session todos |

## Plan mode

| Tool | Purpose |
|---|---|
| `EnterPlanMode` / `ExitPlanMode` | Switch to read-only research mode and back |

## Notes for evolve-loop

- The skill explicitly forbids using the in-process `Agent` tool to spawn Scout/Builder/Auditor in production cycles. Subagents go through `bash scripts/dispatch/subagent-run.sh`, which invokes the per-platform adapter at `scripts/cli_adapters/<cli>.sh`. See SKILL.md's STRICT MODE section.
- `--allowedTools` and `--disallowedTools` flags on `claude -p` accept patterns like `Bash(git status:*)` and `Write(.evolve/runs/cycle-*/*)`. These syntaxes are documented in `.evolve/profiles/*.json`.
- Slash commands like `/evolve-loop` are registered via `.claude-plugin/plugin.json`.
