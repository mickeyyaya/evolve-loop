# Gemini CLI Runtime (Hybrid Driver)

> How `/evolve-loop` reaches the dispatcher under Gemini CLI. **The Claude binary is required at runtime even though Gemini drives the conversation.** This is intentional — see [Why hybrid](#why-hybrid) below.

## Invocation chain

```
User: /evolve-loop 5 polish improve dispatcher
  (typed into Gemini CLI)

  ↓ Gemini resolves the skill via ~/.gemini/extensions/<install-path>
  ↓ activates SKILL.md content; reads reference/platform-detect.md
  ↓ detects platform = gemini, reads reference/gemini-runtime.md (this file)

Skill activates → STRICT MODE: execute exactly one shell command:
  bash scripts/evolve-loop-dispatch.sh 5 polish "improve dispatcher"

  ↓ (Gemini calls run_shell_command)

Dispatcher loops once per cycle:
  bash scripts/run-cycle.sh "improve dispatcher"

  ↓

run-cycle.sh spawns the orchestrator subagent via:
  bash scripts/subagent-run.sh orchestrator $CYCLE $WORKSPACE

  ↓

subagent-run.sh reads .evolve/profiles/orchestrator.json
  → cli = "gemini"  (set by Gemini CLI users via env or profile override)
  → dispatches to scripts/cli_adapters/gemini.sh

  ↓

gemini.sh (HYBRID SHIM):
  1. Probes for `claude` binary on PATH
  2. If found: delegates to scripts/cli_adapters/claude.sh
     (Claude binary becomes the actual runtime engine)
  3. If not found: exits 99 with "install Claude CLI" message

  ↓

claude.sh wraps `claude -p` in sandbox-exec / bwrap, exactly as for the
Claude Code runtime path. Builder, Auditor, Scout all run as Claude
subprocesses with profile-scoped tool permissions.
```

## Why hybrid

Gemini CLI lacks three primitives evolve-loop's runtime depends on:

| Primitive | Gemini status | What breaks without it |
|---|---|---|
| Non-interactive prompt mode (`gemini -p`) | Not supported as of 2026-04 | Cannot spawn isolated subagent sessions |
| `--max-budget-usd` cost cap | Not supported | Runaway cycles can rack up unbounded cost |
| Subagent / Task tool with profile-scoped permissions | Not supported | Builder/Auditor cannot be sandboxed; the kernel hooks have nothing to gate |

The forgery precedent ([docs/incidents/gemini-forgery.md](../../../docs/incidents/gemini-forgery.md)) shows what happens when you run evolve-loop directly on Gemini without these primitives: artifact fabrication, hallucinated git history, forged ledger entries. The kernel hooks (`role-gate`, `ship-gate`, `phase-gate-precondition`) exist *because* of that incident — but they fire on Claude Code's PreToolUse mechanism. Gemini doesn't have the same hook surface.

The hybrid shim keeps the entire Claude-Code trust boundary intact: every Builder edit, every git commit, every `subagent-run.sh` invocation is gated by Claude's PreToolUse hooks. Gemini provides the conversational front-end; Claude provides the isolated execution back-end.

## Required environment

| Variable | Required | Purpose |
|---|---|---|
| `gemini` binary on PATH | yes | The conversational driver |
| `claude` binary on PATH | **yes** | The actual execution engine. Hybrid driver delegates to `claude -p` |
| `ANTHROPIC_API_KEY` | when not in a logged-in Claude session | Auth for the underlying `claude -p` |
| `GEMINI_API_KEY` | yes | Auth for Gemini CLI itself |

## Verifying the hybrid path before running cycles

```bash
# 1. Confirm both binaries are present
command -v gemini && command -v claude

# 2. Confirm gemini.sh delegates correctly (validate-only mode)
VALIDATE_ONLY=1 bash scripts/cli_adapters/gemini.sh
# Expected: exit 0, log line "[gemini-adapter] hybrid-mode: delegating to claude.sh"

# 3. Smoke-test detection
bash scripts/detect-cli.sh
# Expected: prints "gemini" if you're in a Gemini session

# 4. Run the contract test
bash scripts/gemini-adapter-test.sh
# Expected: green
```

If step 2 fails with exit 99, the message will tell you which dependency is missing.

## What if I want true Gemini-driven phases?

Currently unsupported. Tracking in [docs/platform-compatibility.md](../../../docs/platform-compatibility.md) under "Tier 2 / deferred". Two upstream conditions must hold before this becomes safe:

1. Gemini CLI ships non-interactive prompt mode (so subagent dispatch is structurally possible).
2. Read-only Gemini phases (Scout, Evaluator) are demonstrably immune to artifact-fabrication attacks against downstream phases.

Until then, the hybrid driver is the only supported path.

## See also

- [reference/gemini-tools.md](gemini-tools.md) — tool name translation map
- [reference/platform-detect.md](platform-detect.md) — how the skill identifies its platform
- [reference/claude-runtime.md](claude-runtime.md) — what gemini.sh delegates to
- [docs/incidents/gemini-forgery.md](../../../docs/incidents/gemini-forgery.md) — the historical incident this design responds to
