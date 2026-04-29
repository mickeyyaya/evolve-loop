# Codex CLI Tool Names

> Translation map for Claude Code tool names → Codex CLI (formerly OpenAI Codex CLI). Codex runtime support in evolve-loop is currently a stub (`scripts/cli_adapters/codex.sh` exits 99). This file exists for content-portability only.

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

`scripts/cli_adapters/codex.sh` is a deliberate stub that exits 99. `scripts/codex-adapter-test.sh` pins this status so it cannot be silently bypassed. Implementing real codex support requires:

1. Mapping evolve-loop profile fields (`allowed_tools[]`, `disallowed_tools[]`, `max_budget_usd`, `permission_mode`, `add_dir`, `extra_flags`) to the Codex CLI flag surface.
2. Either providing an external budget cap (since `--max-budget-usd` doesn't exist) or accepting unbounded-cost runs.
3. Verifying that Codex's permission/approval model can express the same per-phase access patterns Claude profiles encode.

Until then, set the profile's `cli` field to `claude` (or `gemini` for the hybrid driver). See [docs/platform-compatibility.md](../../../docs/platform-compatibility.md).
