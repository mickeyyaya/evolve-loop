# Platform Detection

> Identify which CLI is running this skill, then load the matching tools and runtime overlay. Read this BEFORE following SKILL.md's invocation steps.

## Detection table (env-var probes, priority order)

| Probe | Match | Conclude | Then read |
|---|---|---|---|
| `EVOLVE_PLATFORM` | any non-empty value | use that value verbatim | `reference/<value>-tools.md` + `reference/<value>-runtime.md` |
| `CLAUDE_CODE_INTERACTIVE` | set | `claude` | `reference/claude-tools.md` + `reference/claude-runtime.md` |
| `CLAUDE_CODE_SESSION_ID` | set | `claude` | same as above |
| `GEMINI_CLI` | set | `gemini` | `reference/gemini-tools.md` + `reference/gemini-runtime.md` |
| `GEMINI_API_KEY` | set AND `claude` not detected | `gemini` | same as above |
| `CODEX_HOME` or `CODEX_API_KEY` | set | `codex` | `reference/codex-tools.md` + `reference/generic-runtime.md` (codex runtime is stub) |
| (none of the above) | — | `unknown` | `reference/generic-runtime.md` |

## Helper script

If you have shell access, the canonical detection happens in `scripts/dispatch/detect-cli.sh`:

```bash
bash scripts/dispatch/detect-cli.sh        # prints one of: claude, gemini, codex, unknown
EVOLVE_PLATFORM=gemini bash scripts/dispatch/detect-cli.sh   # honours the override; prints: gemini
```

This script is platform-neutral — any CLI that can run bash can call it.

## Why detect at skill entry

evolve-loop has two surfaces:

1. **Skill content** — phases, state schema, audit logic. Platform-neutral.
2. **Runtime** — how cycles actually execute. CLI-specific, lives in `scripts/cli_adapters/<cli>.sh`.

The skill content references tools by their **Claude Code names** (`Skill`, `Bash`, `TaskCreate`, etc.) because that's the project's primary platform. When you're on a different CLI, you need a translation layer:

- `reference/<platform>-tools.md` translates tool names (e.g. CC `Bash` → Gemini `run_shell_command`).
- `reference/<platform>-runtime.md` translates invocation patterns (e.g. how `/evolve-loop` is reached on this CLI).

Without reading these overlays first, you may try to invoke a tool that doesn't exist on your platform.

## What "unknown" means

If no probe matches, the skill is running on a CLI without an established adapter. You can still:

- Read SKILL.md and phase docs (purely informational).
- Run `bash scripts/dispatch/evolve-loop-dispatch.sh ...` directly if your platform has shell access — it does not require a specific CLI to be the caller.

You **cannot**:

- Trust the kernel hooks (`role-gate`, `ship-gate`, `phase-gate-precondition`) to fire — they hook into Claude Code's PreToolUse mechanism. Other CLIs may have different hook surfaces.
- Use the `Skill` / `Agent` / `TaskCreate` tools by those names — translate via the closest matching `<platform>-tools.md` if one exists, otherwise stop and ask the user.

See [docs/platform-compatibility.md](../../../docs/platform-compatibility.md) for the full tier matrix.
