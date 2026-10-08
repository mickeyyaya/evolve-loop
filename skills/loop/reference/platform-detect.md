# Platform Detection

> Identify which CLI is running this skill, then load the matching tools and runtime overlay. Read this BEFORE following SKILL.md's invocation steps.

## Detection table (env-var probes, priority order)

| Probe | Match | Conclude | Then read |
|---|---|---|---|
| `CLAUDE_CODE_INTERACTIVE` | set | `claude` | `reference/claude-tools.md` + `reference/claude-runtime.md` |
| `CLAUDE_CODE_SESSION_ID` | set | `claude` | same as above |
| `GEMINI_CLI` | set | `gemini` | `reference/gemini-tools.md` + `reference/gemini-runtime.md` |
| `GEMINI_API_KEY` | set AND `claude` not detected | `gemini` | same as above |
| `CODEX_HOME` or `CODEX_API_KEY` | set | `codex` | `reference/codex-tools.md` + `reference/codex-runtime.md` |
| `agy` binary on PATH | found | `antigravity` | `reference/agy-tools.md` + `reference/agy-runtime.md` |
| (none of the above) | — | `unknown` | `reference/generic-runtime.md` |

## Helper script

If you have shell access, the canonical detection lives in the native Go binary:

```bash
evolve detect-cli                        # prints one of: claude, gemini, codex, antigravity, unknown
```

This subcommand is platform-neutral — any CLI that can run the binary can call it.

## Why detect at skill entry

evolve-loop has two surfaces:

1. **Skill content** — phases, state schema, audit logic. Platform-neutral.
2. **Runtime** — how cycles actually execute. CLI-specific, lives in the Go bridge drivers (`go/internal/bridge/driver_*.go`) and their manifests (`go/internal/bridge/manifests/`).

The skill content references tools by their **Claude Code names** (`Skill`, `Bash`, `TaskCreate`, etc.) because that's the project's primary platform. When you're on a different CLI, you need a translation layer:

- `reference/<platform>-tools.md` translates tool names (e.g. CC `Bash` → Gemini `run_shell_command`).
- `reference/<platform>-runtime.md` translates invocation patterns (e.g. how `/evo:loop` is reached on this CLI).

Without reading these overlays first, you may try to invoke a tool that doesn't exist on your platform.

## What "unknown" means

If no probe matches, the skill is running on a CLI without an established adapter. You can still:

- Read SKILL.md and phase docs (purely informational).
- Run `evolve loop ...` directly if your platform has shell access (the STRICT MODE command in SKILL.md). It does not require a specific CLI to be the caller.

You **cannot**:

- Trust the kernel hooks (`evolve guard role`, `evolve guard ship`, `evolve guard phase`) to fire — they hook into Claude Code's PreToolUse mechanism. Other CLIs may have different hook surfaces.
- Use the `Skill` / `Agent` / `TaskCreate` tools by those names — translate via the closest matching `<platform>-tools.md` if one exists, otherwise stop and ask the user.

See [docs/architecture/platform-compatibility.md](../../../docs/architecture/platform-compatibility.md) for the current CLI support.
