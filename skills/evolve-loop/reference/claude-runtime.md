# Claude Code Runtime

> How `/evolve-loop` reaches the dispatcher under Claude Code. This is the reference runtime — every gate and adapter assumes Claude semantics first.

## Invocation chain

```
User: /evolve-loop 5 polish improve dispatcher

  ↓ Claude Code resolves the slash command
  ↓ via .claude-plugin/plugin.json → skills/evolve-loop/SKILL.md

Skill activates → STRICT MODE: execute exactly one Bash command:
  bash scripts/dispatch/evolve-loop-dispatch.sh 5 polish "improve dispatcher"

  ↓

Dispatcher loops once per cycle:
  bash scripts/dispatch/run-cycle.sh "improve dispatcher"

  ↓

run-cycle.sh spawns the orchestrator subagent:
  bash scripts/dispatch/subagent-run.sh orchestrator $CYCLE $WORKSPACE

  ↓

subagent-run.sh reads .evolve/profiles/orchestrator.json
  → cli = "claude"
  → dispatches to scripts/cli_adapters/claude.sh

  ↓

claude.sh wraps `claude -p` in sandbox-exec (macOS) or bwrap (Linux),
applies --allowedTools / --disallowedTools / --max-budget-usd / etc.

  ↓

The orchestrator subagent invokes Scout, Builder, Auditor in turn —
each via the same subagent-run.sh path with their own profile.
```

## Required environment

| Variable | Required | Purpose |
|---|---|---|
| `claude` binary on PATH | yes | The runtime engine. Verify with `command -v claude`. |
| `ANTHROPIC_API_KEY` | when running outside a logged-in Claude session | Auth for `claude -p`. The `--bare` flag in profiles strips other auth sources. |
| `CLAUDE_CODE_INTERACTIVE` | set automatically by Claude Code | Used by `scripts/dispatch/detect-cli.sh` to identify the platform. |

Optional but recommended:

| Variable | Purpose |
|---|---|
| `EVOLVE_SANDBOX=1` | Enable kernel-level sandbox-exec / bwrap wrapping (default in profiles with `sandbox.enabled: true`) |
| `EVOLVE_TASK_MODE=research` (or `deep`) | Select a budget tier from the profile's `budget_tiers` map |
| `EVOLVE_MAX_BUDGET_USD=1.50` | Override the per-invocation budget (highest precedence) |

## Trust boundary

Three PreToolUse kernel hooks fire on Claude Code:

| Hook | Job |
|---|---|
| `scripts/hooks/ship-gate.sh` | Only `scripts/lifecycle/ship.sh` may execute git commit/push/gh release |
| `scripts/hooks/role-gate.sh` | Edit/Write must match the active phase's path allowlist |
| `scripts/hooks/phase-gate-precondition.sh` | `subagent-run.sh` invocations must follow Scout → Builder → Auditor sequence per `.evolve/cycle-state.json` |

These hooks are configured in `.claude-plugin/plugin.json`. They are the structural enforcement of the trust boundary — the orchestrator cannot edit source directly, cannot push without going through `ship.sh`, and cannot skip phases.

## When to NOT use the strict dispatcher

The dispatcher is mandatory for `/evolve-loop` invocations. The only documented exception is `EVOLVE_DISPATCH_VERIFY=0`, which skips per-cycle ledger verification — used solely for debugging the dispatcher itself. Setting it for real cycles disables the only structural enforcement of pipeline completeness; do not.

## Failure modes

| Dispatcher exit | Meaning | Follow-up |
|---|---|---|
| `0` | All cycles ran AND ledger verified end-to-end | Report summary; done |
| `1` | A `run-cycle.sh` invocation failed | Surface the cycle's stderr; do NOT retry inline |
| `2` | A cycle bypassed Scout/Builder/Auditor (CRITICAL) | Quote ledger counts; recommend `git log` of `.evolve/runs/cycle-N/`; STOP |
| `10` | Bad CLI arguments | Re-prompt with valid args |
| `99` | Provider not supported (from a `cli_adapters/<cli>.sh` stub) | Switch the offending profile's `cli` field; do not "fix" the stub blindly |

## See also

- SKILL.md STRICT MODE section
- [reference/claude-tools.md](claude-tools.md) — tool names this runtime expects
- [docs/release-protocol.md](../../../docs/release-protocol.md) — the publish vs push vs ship distinction
