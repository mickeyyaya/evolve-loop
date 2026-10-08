# Codex CLI Tool Names

> Translation map for Claude Code tool names → Codex CLI (formerly OpenAI Codex CLI). Codex dispatches via the native Go bridge drivers (`codex` / `codex-tmux`); the former bash adapter was removed in the script→Go migration (2026-06). This file is the tool-name translation map.

## Direct equivalents

| Claude Code | Codex CLI |
|---|---|
| `Read` | `read` |
| `Write` | `write` |
| `Edit` | `edit` |
| `Bash` | `shell` |
| `Grep` | `search` |
| `Glob` | `find` |
| `WebSearch` | `web.search` |
| `WebFetch` | `web.fetch` |

## No equivalent (gaps)

| Claude Code | Codex CLI |
|---|---|
| `Skill` (registered skill activation) | **None native** — skills are loaded via `~/.codex/agents/` config but invoked inline rather than via a dedicated tool |
| `Agent` / `Task` (subagent dispatch) | **None.** Same single-session limitation as Gemini |
| `--max-budget-usd` flag | **None** in current CLI |
| `--allowedTools` / `--disallowedTools` syntax | **Different.** Codex uses an `approval-policy` config file with allow/deny rules at higher granularity |
| `EnterPlanMode` / `ExitPlanMode` | **None native** — plan mode is implicit when read-only ops are selected |

## Runtime status in evolve-loop

Codex runs through two native bridge drivers. The `codex` driver runs `codex exec --output-last-message <artifact>` (`go/internal/bridge/driver_codex.go`). The `codex-tmux` driver runs the codex TUI in tmux (`go/internal/bridge/driver_codextmux.go`). Codex has no `--permission-mode` flag, so the `codex` driver refuses a profile `permission_mode` and does not ignore it.

The checked-in routing table (`.evolve/policy.json` `cli_routing`) does not list codex, so a phase runs on codex only under `--bypass-policy`. See [reference/codex-runtime.md](codex-runtime.md) and [docs/architecture/platform-compatibility.md](../../../docs/architecture/platform-compatibility.md).
