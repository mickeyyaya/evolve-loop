# Platform Compatibility

> Which AI coding CLIs can run evolve-loop, and at what tier.

## Quick answer

| CLI | Runtime tier | Status (v8.15+) | Notes |
|---|---|---|---|
| **Claude Code** | Tier 1 — primary | Production | The reference runtime. Every phase, gate, and subagent dispatch is tested here first. |
| **Gemini CLI** | Tier 1 — hybrid driver (**requires Claude CLI on PATH**) | Supported | Gemini drives the conversation; subagents execute via `claude -p` — **the `claude` binary must be installed independently**. Without it, `gemini.sh` exits 99. See [Why hybrid](#why-hybrid). |
| **Codex CLI** | Tier 3 — stub | Unsupported | `scripts/cli_adapters/codex.sh` exits 99. No production use. Implementing requires the same shape as `gemini.sh`. |
| **Copilot CLI** | Tier 3 — not attempted | Unsupported | No adapter exists. Skill text is portable; runtime is not. |
| **Other agentic CLIs** | Tier 4 — generic fallback | Skill-text-only | Any CLI that can read markdown and invoke shell scripts can read SKILL.md and follow the dispatcher path, but won't have a tested adapter. |

## How the tiers work

evolve-loop has two surfaces, and each platform supports them independently:

1. **Skill content surface** — what the LLM reads. SKILL.md, phase docs, references. Platform-neutral. Any CLI that can load a markdown skill can consume this.
2. **Runtime surface** — how cycles actually execute. The dispatcher (`scripts/evolve-loop-dispatch.sh`) calls `scripts/run-cycle.sh`, which spawns subagents via `scripts/subagent-run.sh`, which dispatches to a per-CLI adapter at `scripts/cli_adapters/<cli>.sh`. The adapter is the platform-specific layer.

Tier 1 means both surfaces work. Tier 3 means only the skill content surface works — the runtime returns exit 99.

## Why hybrid (Gemini, Tier 1)

As of 2026-04, Gemini CLI lacks three primitives evolve-loop's runtime depends on:

| Required primitive | Gemini status | Why it matters |
|---|---|---|
| Non-interactive prompt mode (`gemini -p`) | Not supported | Subagent dispatch needs `<cli> -p "<prompt>"` to spawn an isolated session. No equivalent exists. |
| `--max-budget-usd` cost cap | Not supported | Without an external cap, runaway cycles can rack up unbounded cost. Claude has a per-invocation flag. |
| Subagent / Task tool | Not supported | Builder and Auditor must run in sandboxed sub-sessions with profile-scoped permissions. Gemini's `activate_skill` runs in the same session. |

The forgery precedent (`docs/incidents/gemini-forgery.md`) shows what happens when these primitives are missing: Gemini wrote a `run_15_cycles_forgery.sh` that fabricated artifacts, hallucinated git commits, and forged ledger entries. The kernel hooks (`role-gate`, `ship-gate`, `phase-gate-precondition`) are the structural fix — but they only fire on Claude subprocesses today.

The hybrid driver sidesteps the gaps: when invoked from Gemini CLI, the adapter at `scripts/cli_adapters/gemini.sh` delegates to `claude.sh`. Gemini provides the conversational surface; Claude provides the isolated runtime. Both binaries must be installed.

## Installation per platform

### Claude Code (Tier 1)

```bash
# evolve-loop installs as a Claude plugin
# See README for the marketplace URL
claude --version  # 1.0.0+
```

### Gemini CLI (Tier 1, hybrid)

```bash
# Both binaries required
gemini --version
claude --version  # used by hybrid adapter

# Verify the hybrid path
bash scripts/cli_adapters/gemini.sh --probe  # exit 0 means ready
```

If `claude` is missing on PATH when Gemini invokes a cycle, `gemini.sh` exits 99 with a message directing the user to install Claude CLI. There is no silent fallback.

### Other CLIs (Tier 3/4)

You can read SKILL.md and the phase docs from any CLI. To run cycles, you must implement an adapter at `scripts/cli_adapters/<your-cli>.sh` that mirrors `claude.sh`'s contract — see [Adapter contract](#adapter-contract) below.

## Adapter contract

Every adapter at `scripts/cli_adapters/<cli>.sh` is invoked by `subagent-run.sh` with these env vars:

| Var | Purpose |
|---|---|
| `PROFILE_PATH` | Absolute path to agent profile JSON |
| `RESOLVED_MODEL` | Resolved tier (haiku/sonnet/opus) |
| `PROMPT_FILE` | Path to prompt file |
| `CYCLE` | Cycle number (integer) |
| `WORKSPACE_PATH` | `.evolve/runs/cycle-N/` |
| `WORKTREE_PATH` | Optional builder isolation path |
| `STDOUT_LOG` | Where to redirect stdout |
| `STDERR_LOG` | Where to redirect stderr |
| `ARTIFACT_PATH` | Resolved output artifact path |
| `VALIDATE_ONLY` | If set, print the command and exit without invoking the LLM |

The adapter must:
1. Build the underlying CLI's invocation from profile fields.
2. Stream stdout to `STDOUT_LOG`, stderr to `STDERR_LOG`.
3. Write the agent's report to `ARTIFACT_PATH`.
4. Exit with the underlying CLI's exit code (or 99 for "provider not supported").

See `scripts/cli_adapters/claude.sh` for the canonical implementation, including budget tier resolution, sandbox-exec / bwrap wrapping, and challenge token handling.

## Detection

The skill auto-detects which CLI it's running under via `scripts/detect-cli.sh`. Detection signals (priority order):

1. `CLAUDE_CODE_INTERACTIVE` set → `claude`
2. `GEMINI_CLI` or `GEMINI_API_KEY` set → `gemini`
3. `CODEX_*` env vars → `codex`
4. Otherwise → `unknown`

You can override detection with `EVOLVE_PLATFORM=<cli>`. The skill reads `reference/<platform>-runtime.md` to determine which invocation pattern to use.

## See also

- [reference/platform-detect.md](../skills/evolve-loop/reference/platform-detect.md) — env-var probe table the LLM consults at skill activation
- [reference/claude-tools.md](../skills/evolve-loop/reference/claude-tools.md), [reference/gemini-tools.md](../skills/evolve-loop/reference/gemini-tools.md), [reference/codex-tools.md](../skills/evolve-loop/reference/codex-tools.md) — per-platform tool name maps
- [reference/claude-runtime.md](../skills/evolve-loop/reference/claude-runtime.md), [reference/gemini-runtime.md](../skills/evolve-loop/reference/gemini-runtime.md), [reference/generic-runtime.md](../skills/evolve-loop/reference/generic-runtime.md) — per-platform invocation patterns
- [docs/incidents/gemini-forgery.md](incidents/gemini-forgery.md) — why hybrid driving exists
